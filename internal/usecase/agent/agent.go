// Package agent runs the AI Wizard: a model-driven import of a character
// sheet, with the server supplying the tools the model builds the draft with.
//
// The import coordinator owns one isolated draft per session. External model
// calls never hold its mutex; a generation check fences their late results.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type AgentCall struct{ ID, Name, Arguments string }

type AgentResponse struct {
	Text   string
	Output []json.RawMessage
	Calls  []AgentCall
	Usage  AgentUsage
}

// AgentUsage is what one model request was billed for. Cached is the part of
// Input the provider served from its prompt cache.
type AgentUsage struct {
	Input  int64 `json:"input"`
	Cached int64 `json:"cached"`
	Output int64 `json:"output"`
}

type AgentRequest struct {
	Input  []json.RawMessage
	Files  []AgentFile
	Locale string
	// Session keys the provider's prompt cache: every request of one session
	// repeats the same sources, instructions and tools.
	Session    string
	Unattended bool
}

type AgentModel interface {
	Respond(context.Context, AgentRequest, func(string)) (AgentResponse, error)
}

type AgentConfig struct {
	Workers, MaxTurns, MaxSessions int
	Timeout                        time.Duration
	// Store is where sessions are kept; nil keeps them in this process.
	Store Store
}

// Agent runs the wizard's turns. It keeps no session of its own: each one
// lives in the Store, a turn is worked on by whichever instance claimed it,
// and every reader is answered from the Store. See docs/agent.md.
type Agent struct {
	store Store
	// instance names this process in a lease. A new one each start: a turn
	// left running by the last process is taken over when its lease runs out.
	instance string
	// local guards running.
	local sync.Mutex
	// running is the turns this process has in flight, and how to stop each.
	running map[string]context.CancelFunc
	service *charuc.Service
	model   AgentModel
	config  AgentConfig
	wake    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

const (
	// leaseMargin is how long past a turn's own deadline its lease runs.
	// ponytail: no heartbeat, so a crashed server's turn waits out the whole
	// lease (the request timeout plus this); add one if that wait matters.
	leaseMargin = 30 * time.Second
	// sessionIdle is how long an unused chat is kept.
	sessionIdle   = 24 * time.Hour
	sweepInterval = 10 * time.Minute
	storeTimeout  = 10 * time.Second
)

func NewAgent(service *charuc.Service, model AgentModel, cfg AgentConfig) *Agent {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 40
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 100
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Minute
	}
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore()
	}
	var token [16]byte
	_, _ = rand.Read(token[:])
	ctx, cancel := context.WithCancel(context.Background())
	a := &Agent{store: cfg.Store, instance: hex.EncodeToString(token[:]), running: map[string]context.CancelFunc{}, service: service, model: model, config: cfg, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel}
	for i := 0; i < cfg.Workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	a.wg.Add(1)
	go a.tick()
	return a
}

// tick is what makes several processes one wizard: every second a worker
// looks for a turn queued elsewhere or left by a process that died, and every
// sweepInterval the chats nobody has used for a day are deleted.
func (a *Agent) tick() {
	defer a.wg.Done()
	look, sweep := time.NewTicker(time.Second), time.NewTicker(sweepInterval)
	defer look.Stop()
	defer sweep.Stop()
	a.signal()
	for {
		if _, err := a.store.Sweep(a.ctx, sessionIdle); err != nil && a.ctx.Err() == nil {
			a.service.Logger().Warn("AI wizard sweep failed", "error", err)
		}
	wait:
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-look.C:
				a.signal()
			case <-sweep.C:
				break wait
			}
		}
	}
}

func (a *Agent) Close() { a.cancel(); a.wg.Wait() }

func (a *Agent) Enabled() bool { return a.model != nil }

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func (a *Agent) signal() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}
