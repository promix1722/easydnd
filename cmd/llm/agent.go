package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	agentmodel "github.com/promix1722/easydnd/internal/adapter/agent/openai"
	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/config"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
	agentuc "github.com/promix1722/easydnd/internal/usecase/agent"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type agentCommand struct {
	Action string   `json:"action"`
	Files  []string `json:"files"`
	Text   string   `json:"text"`
	// Unattended starts a session that asks nothing and leaves open what the
	// sources do not state.
	Unattended bool `json:"unattended"`
	Revision   int  `json:"revision"`
}

// Writes from the model worker and the command loop share one JSON stream.
type agentOutput struct {
	mu     sync.Mutex
	writer io.Writer
}

func (o *agentOutput) emit(v any) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return json.NewEncoder(o.writer).Encode(v)
}

type timedAgentModel struct {
	agentuc.AgentModel
	out *agentOutput
}

func (m timedAgentModel) Respond(ctx context.Context, r agentuc.AgentRequest, delta func(string)) (agentuc.AgentResponse, error) {
	start := time.Now()
	response, err := m.AgentModel.Respond(ctx, r, delta)
	_ = m.out.emit(map[string]any{"kind": "model", "durationMs": time.Since(start).Milliseconds(), "calls": response.Calls, "text": response.Text, "usage": response.Usage, "failed": err != nil})
	return response, err
}

func agentCmd(args []string) error {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	pack := flags.String("pack", "data/srd_5.1", "pack directory or portable JSON")
	configPath := flags.String("config", "config.local.yaml", "existing EasyDND configuration (agent settings)")
	locale := flags.String("locale", "en", "catalogue locale")
	timeout := flags.Duration("timeout", 5*time.Minute, "maximum time per start/message/resume")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *timeout <= 0 {
		return fmt.Errorf("unexpected arguments or invalid timeout")
	}
	lang := rules.Locale(*locale)
	if !lang.IsSupported() {
		return fmt.Errorf("unsupported locale %q", *locale)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if cfg.Agent.APIKey == "" || cfg.Agent.Model == "" {
		return fmt.Errorf("configure agent.api_key and agent.model in %s", *configPath)
	}
	source, err := catalogfile.NewRegistry([]string{*pack}, nil, "")
	if err != nil {
		return err
	}
	if _, err = source.Load(context.Background(), lang); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	service := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), source, nil, nil, logger)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return runAgentCLI(ctx, os.Stdin, os.Stdout, service, agentmodel.New(cfg.Agent.APIKey, cfg.Agent.Model, cfg.Agent.ReasoningEffort), lang, agentuc.AgentConfig{Workers: 1, MaxTurns: cfg.Agent.MaxTurns, Timeout: cfg.Agent.RequestTimeout}, *timeout)
}

func agentFiles(paths []string) ([]agentuc.AgentFile, error) {
	if len(paths) > 8 {
		return nil, fmt.Errorf("at most eight attachments")
	}
	files := []agentuc.AgentFile{}
	total := 0
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, (20<<20)+1))
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		total += len(data)
		if total > 20<<20 {
			return nil, fmt.Errorf("attachments exceed 20 MiB")
		}
		mime := http.DetectContentType(data)
		switch {
		case strings.HasPrefix(mime, "text/plain"):
			if !utf8.Valid(data) || len(data) > 256<<10 {
				return nil, fmt.Errorf("invalid or oversized UTF-8 attachment")
			}
			mime = "text/plain"
			if json.Valid(data) {
				mime = "application/json"
			}
		case mime == "application/pdf", mime == "image/png", mime == "image/jpeg", mime == "image/webp":
		default:
			return nil, fmt.Errorf("unsupported attachment %s", path)
		}
		files = append(files, agentuc.AgentFile{Name: filepath.Base(path), MIME: mime, Data: data})
	}
	return files, nil
}

func runAgentCLI(ctx context.Context, input io.Reader, output io.Writer, service *charuc.Service, model agentuc.AgentModel, locale rules.Locale, cfg agentuc.AgentConfig, timeout time.Duration) error {
	out := &agentOutput{writer: output}
	agent := agentuc.NewAgent(service, timedAgentModel{model, out}, cfg)
	defer agent.Close()
	const owner domain.OwnerID = "cli"
	lines := make(chan string)
	readError := make(chan error, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-done:
				return
			}
		}
		readError <- scanner.Err()
		close(lines)
	}()
	var id string
	var deadline time.Time
	seen, revision := 0, -1
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	inspect := func(s agentuc.AgentSession, action string) error {
		state, err := agent.Sheet(ctx, s)
		if err != nil {
			return err
		}
		cat, err := agent.Catalog(ctx, s)
		if err != nil {
			return err
		}
		prompts, err := domain.Prompts(s.Log, cat)
		if err != nil {
			return err
		}
		return out.emit(map[string]any{"kind": "result", "action": action, "session": s, "sheet": state, "log": s.Log, "prompts": prompts, "rules": s.Log.RulesLock()})
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-lines:
			if !ok {
				return <-readError
			}
			var command agentCommand
			err := json.Unmarshal([]byte(line), &command)
			var s agentuc.AgentSession
			if err == nil {
				switch command.Action {
				case "start":
					if id != "" {
						err = fmt.Errorf("one session per process")
						break
					}
					var files []agentuc.AgentFile
					files, err = agentFiles(command.Files)
					if err == nil {
						create := agent.Create
						if command.Unattended {
							create = agent.CreateUnattended
						}
						s, err = create(ctx, owner, "", locale, files, command.Text)
					}
					if err == nil {
						id = s.ID
						deadline = time.Now().Add(timeout)
					}
				case "inspect":
					s, err = agent.Get(owner, id)
				case "message", "resume", "retry", "stop", "finish", "discard":
					s, err = agent.Control(owner, id, command.Action, command.Text, command.Revision)
					if err == nil && (command.Action == "message" || command.Action == "resume" || command.Action == "retry") {
						deadline = time.Now().Add(timeout)
					}
				default:
					err = fmt.Errorf("unknown action %q", command.Action)
				}
			}
			if err == nil {
				if command.Action == "discard" {
					err = out.emit(map[string]any{"kind": "result", "action": "discard", "session": s})
					id = ""
					seen, revision = 0, -1
					deadline = time.Time{}
				} else {
					err = inspect(s, command.Action)
				}
			}
			if err != nil {
				if e := out.emit(map[string]any{"kind": "error", "action": command.Action, "error": err.Error()}); e != nil {
					return e
				}
			}
		case <-ticker.C:
			if id == "" {
				continue
			}
			s, err := agent.Get(owner, id)
			if err != nil {
				return err
			}
			if !deadline.IsZero() && time.Now().After(deadline) && (s.Status == "queued" || s.Status == "running") {
				s, err = agent.Control(owner, id, "stop", "", s.Revision)
				if err != nil {
					return err
				}
				if err = out.emit(map[string]any{"kind": "timeout"}); err != nil {
					return err
				}
				deadline = time.Time{}
			}
			for _, event := range s.Events[seen:] {
				if err = out.emit(map[string]any{"kind": "event", "event": event}); err != nil {
					return err
				}
			}
			seen = len(s.Events)
			if revision != s.Revision {
				revision = s.Revision
				if err = out.emit(map[string]any{"kind": "status", "status": s.Status, "revision": s.Revision}); err != nil {
					return err
				}
			}
		}
	}
}
