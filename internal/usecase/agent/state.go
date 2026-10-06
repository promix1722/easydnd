package agent

import "fmt"

// transitions is every move a session's status may make. A status may also
// stay what it is. Who is allowed to make a move is not decided here but by
// the store: a turn's writes are accepted only from the instance holding the
// lease, at the generation it claimed. See docs/agent.md#session-lifetime.
var transitions = map[string][]string{
	// The owner's first message, once the rules are chosen.
	"opening": {"queued"},
	// A worker claims it; stop and finish pause it before one does.
	"queued": {"running", "paused"},
	// The lease holder ends the turn; released back to queued on shutdown, or
	// by the owner's next message, which fences the turn in flight.
	"running": {"queued", "waiting", "review", "paused", "failed"},
	"waiting": {"queued", "paused"},
	"review":  {"queued", "paused"},
	"paused":  {"queued"},
	"failed":  {"queued", "paused"},
}

// setStatus moves a session along transitions and refuses anything else.
func setStatus(s *AgentSession, to string) error {
	if s.Status == to {
		return nil
	}
	for _, allowed := range transitions[s.Status] {
		if allowed == to {
			s.Status = to
			return nil
		}
	}
	return fmt.Errorf("agent session cannot go from %q to %q", s.Status, to)
}
