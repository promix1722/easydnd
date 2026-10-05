package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	catalogfile "github.com/promix1722/easydnd/internal/adapter/catalog/file"
	"github.com/promix1722/easydnd/internal/adapter/repository/memory"
	"github.com/promix1722/easydnd/internal/domain/rules"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

type cliTestModel func(context.Context, charuc.AgentRequest, func(string)) (charuc.AgentResponse, error)

func (f cliTestModel) Respond(ctx context.Context, r charuc.AgentRequest, delta func(string)) (charuc.AgentResponse, error) {
	return f(ctx, r, delta)
}

func TestAgentCLIConversationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			service := charuc.NewService(memory.NewCharacterRepository(), memory.NewFolderRepository(), catalogfile.NewSource("../../data/srd_5.1"), nil, nil, slog.New(slog.DiscardHandler))
			turn := 0
			model := cliTestModel(func(ctx context.Context, r charuc.AgentRequest, _ func(string)) (charuc.AgentResponse, error) {
				if deadline {
					<-ctx.Done()
					return charuc.AgentResponse{}, ctx.Err()
				}
				turn++
				if turn == 1 {
					return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "ask", Name: "ask_user", Arguments: `{"text":"Name?","options":["Hero"]}`}}}, nil
				}
				if !strings.Contains(string(r.Input[len(r.Input)-1]), "Hero") {
					t.Error("reply was not passed to model")
				}
				return charuc.AgentResponse{Calls: []charuc.AgentCall{{ID: "ask-again", Name: "ask_user", Arguments: `{"text":"Next?","options":["Done"]}`}}}, nil
			})
			in, send := io.Pipe()
			output, write := io.Pipe()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- runAgentCLI(ctx, in, write, service, model, rules.DefaultLocale, charuc.AgentConfig{Workers: 1}, 150*time.Millisecond)
				_ = write.Close()
			}()
			messages := make(chan map[string]json.RawMessage, 100)
			go func() {
				scan := bufio.NewScanner(output)
				scan.Buffer(make([]byte, 4096), 4<<20)
				for scan.Scan() {
					var v map[string]json.RawMessage
					if err := json.Unmarshal(scan.Bytes(), &v); err != nil {
						t.Error(err)
					}
					messages <- v
				}
				close(messages)
			}()
			command := func(s string) {
				t.Helper()
				if _, err := fmt.Fprintln(send, s); err != nil {
					t.Fatal(err)
				}
			}
			wait := func(kind string) map[string]json.RawMessage {
				t.Helper()
				for {
					select {
					case v, ok := <-messages:
						if !ok {
							t.Fatal("output closed")
						}
						if string(v["kind"]) == `"`+kind+`"` {
							return v
						}
					case <-ctx.Done():
						t.Fatal("CLI did not respond")
					}
				}
			}
			bad := filepath.Join(t.TempDir(), "bad.bin")
			if err := os.WriteFile(bad, []byte{0, 1, 2, 3}, 0600); err != nil {
				t.Fatal(err)
			}
			command(fmt.Sprintf(`{"action":"start","files":[%q],"text":"Import"}`, bad))
			wait("error")
			command(`{"action":"start","text":"Create a hero"}`)
			wait("result")
			if deadline {
				wait("timeout")
			} else {
				for {
					v := wait("status")
					if string(v["status"]) == `"waiting"` {
						break
					}
				}
				command(`{"action":"message","revision":-1,"text":"Hero"}`)
				wait("error")
				command(`{"action":"inspect"}`)
				result := wait("result")
				var session charuc.AgentSession
				if err := json.Unmarshal(result["session"], &session); err != nil {
					t.Fatal(err)
				}
				if len(result["sheet"]) == 0 || len(result["log"]) == 0 || len(result["prompts"]) == 0 {
					t.Fatal("inspection missing evidence")
				}
				command(fmt.Sprintf(`{"action":"message","revision":%d,"text":"Hero"}`, session.Revision))
				wait("result")
				for {
					v := wait("status")
					if string(v["status"]) == `"waiting"` {
						break
					}
				}
			}
			_ = send.Close()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
