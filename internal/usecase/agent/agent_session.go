package agent

import (
	"encoding/json"
	"errors"
	"time"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/types"
)

type AgentFile struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Data []byte `json:"-"`
}

type AgentEvent struct {
	Actions    []string        `json:"actions,omitempty"`
	Files      []AgentFile     `json:"files,omitempty"`
	Options    []string        `json:"options,omitempty"`
	Source     string          `json:"source,omitempty"`
	Assumption string          `json:"assumption,omitempty"`
	ID         int             `json:"id"`
	Kind       string          `json:"kind"`
	Text       string          `json:"text,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type AgentManual struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type agentOperation struct {
	Hash   [32]byte
	Result json.RawMessage
	Error  bool
}

type AgentSession struct {
	expected []string
	// nudged is the checklist entries review was last refused over. The same
	// list a second time means the model has seen it and is done.
	nudged []string
	// printed is the sheet's derived numbers -- hit points, armor class, skill
	// and save bonuses -- by fact path, as a model read them. They are what the
	// build is checked against and never part of the character: a number read
	// off a scan is the least reliable thing in an import, and pinning one
	// over a build that computes otherwise makes the sheet wrong with authority.
	printed map[string]int
	// spells is the cantrips and spells the sheet lists, by printed name.
	spells []string
	// prepared is the ones among them the sheet marks as prepared.
	prepared []string
	// scores is the six totals the sheet prints. They are never written to the
	// character: after every tool call the base scores are solved again so the
	// build comes out at them, whatever order race and improvements arrive in.
	// A player editing the character ends that -- their numbers win.
	scores map[rules.Ability]int
	// offered is that review has just been refused over questions the owner
	// was never put, so that the question which follows is known to be about
	// them.
	offered bool
	// questions is everything the owner has been asked, with what it was
	// about and what they answered. It is what keeps a question from being
	// put twice: the model is shown it wherever it decides whether to ask,
	// and review is held back only for what is not in it.
	questions []agentQuestion
	// classLevels is the level the sheet prints beside each class.
	classLevels map[rules.Slug]int
	// characterRevision is the stored character's revision as the agent last
	// read or wrote it. Any other value means the player edited it meanwhile.
	characterRevision int
	operations        map[string]agentOperation
	ID                string          `json:"id"`
	Folder            domain.FolderID `json:"folder"`
	Status            string          `json:"status"`
	Revision          int             `json:"revision"`
	CharacterID       domain.ID       `json:"characterId,omitempty"`
	// Created orders an owner's chats, so the wizard can reopen the latest.
	Created time.Time `json:"created"`
	// Finished is the owner pressing Finish. Until then the wizard reopens
	// this chat, however done the assistant thinks the character is.
	Finished bool `json:"finished"`
	// Unattended is the owner asking, when the chat began, not to be asked
	// anything: what the sources state is imported, the rest is left open and
	// listed at review. It is set with the session and never by the model --
	// a model's own word that the owner chose this was not reliable.
	Unattended  bool              `json:"unattended,omitempty"`
	Events      []AgentEvent      `json:"events"`
	Files       []AgentFile       `json:"files"`
	Manual      []AgentManual     `json:"manual"`
	Assumptions []string          `json:"assumptions"`
	Owner       domain.OwnerID    `json:"-"`
	Locale      rules.Locale      `json:"-"`
	Log         domain.Log        `json:"-"`
	Generation  int               `json:"-"`
	Input       []json.RawMessage `json:"-"`
	Turns       int               `json:"-"`
	// rules is the answer to the opening question, and chosen that it was
	// given: the default rules are an answer too, and their lock is empty.
	rules  pack.Lock
	chosen bool
	// count is how many of Events the store already holds.
	count int
}

// agentQuestion is one question the owner was put, and what came of it.
type agentQuestion struct {
	Text    string
	Options []string
	// About is the open prompts the question put to them, by the id a model
	// reads them under: the ones it named, the ones whose own options it
	// offered as answers, and after a refused review all that was left.
	About []string
	// Open is every prompt that was open when it was asked. The same answers
	// offered again over the same open prompts is the same question.
	Open []string
	// Answer is the owner's reply, once there is one.
	Answer string
}

// replied files a message of the owner's as the answer to the question they
// were last put, if it is still waiting for one.
func (s *AgentSession) replied(text string) {
	if n := len(s.questions); n > 0 && s.questions[n-1].Answer == "" {
		s.questions[n-1].Answer = text
	}
}

// decided is what the owner has already said about each prompt: the last
// answered question that put it to them.
func (s *AgentSession) decided() map[string]agentQuestion {
	out := map[string]agentQuestion{}
	for _, q := range s.questions {
		if q.Answer == "" {
			continue
		}
		for _, id := range q.About {
			out[id] = q
		}
	}
	return out
}

// answers is the owner's choices so far, in the order they were made, as a
// model is shown them.
func (s *AgentSession) answers() []map[string]any {
	out := []map[string]any{}
	for _, q := range s.questions {
		if q.Answer != "" {
			out = append(out, map[string]any{"question": q.Text, "answer": q.Answer, "about": q.About})
		}
	}
	return out
}

func addAgentEvent(s *AgentSession, kind, text, tool string, data any) {
	e := AgentEvent{ID: len(s.Events) + 1, Kind: kind, Text: text, Tool: tool}
	if data != nil {
		e.Data = raw(data)
	}
	s.Events = append(s.Events, e)
}

// document is the part of a session only this package reads back.
type document struct {
	Folder            domain.FolderID
	CharacterID       domain.ID
	Unattended        bool
	Files             []AgentFile
	Manual            []AgentManual
	Assumptions       []string
	Locale            rules.Locale
	Log               domain.Log
	Input             []json.RawMessage
	Turns             int
	Expected, Nudged  []string
	Printed           map[string]int
	Spells, Prepared  []string
	Scores            map[rules.Ability]int
	Offered           bool
	Questions         []agentQuestion
	ClassLevels       map[rules.Slug]int
	CharacterRevision int
	Operations        map[string]agentOperation
	Rules             pack.Lock
	Chosen            bool
}

// encode is a session as the store keeps it. pending are the calls of the
// response being worked through that have not run yet: each is stored as
// answered "not run", because the provider rejects a transcript with an
// unanswered call and whoever continues from the stored one must be able to
// send it.
func encode(s *AgentSession, pending []AgentCall) Record {
	input := s.Input
	if len(pending) > 0 {
		input = append([]json.RawMessage(nil), input...)
		for _, call := range pending {
			input = append(input, raw(map[string]any{"type": "function_call_output", "call_id": call.ID, "output": string(agentError(errInterrupted))}))
		}
	}
	return Record{ID: s.ID, Owner: s.Owner, Status: s.Status, Revision: s.Revision, Generation: s.Generation, Finished: s.Finished, Created: s.Created, Count: s.count, Events: s.Events,
		Document: raw(document{Folder: s.Folder, CharacterID: s.CharacterID, Unattended: s.Unattended, Files: s.Files, Manual: s.Manual, Assumptions: s.Assumptions, Locale: s.Locale, Log: s.Log, Input: input, Turns: s.Turns,
			Expected: s.expected, Nudged: s.nudged, Printed: s.printed, Spells: s.spells, Prepared: s.prepared, Scores: s.scores, Offered: s.offered, Questions: s.questions, ClassLevels: s.classLevels, CharacterRevision: s.characterRevision, Operations: s.operations, Rules: s.rules, Chosen: s.chosen})}
}

var errInterrupted = errors.New("not run: the turn was interrupted; send it again if it is still needed")

func decode(rec Record) (*AgentSession, error) {
	var d document
	if err := json.Unmarshal(rec.Document, &d); err != nil {
		return nil, types.WrapServerError(err, "decode import session")
	}
	s := &AgentSession{ID: rec.ID, Owner: rec.Owner, Status: rec.Status, Revision: rec.Revision, Generation: rec.Generation, Finished: rec.Finished, Created: rec.Created, count: rec.Count,
		Events: append([]AgentEvent{}, rec.Events...), Files: append([]AgentFile{}, d.Files...), Manual: append([]AgentManual{}, d.Manual...), Assumptions: append([]string{}, d.Assumptions...),
		Folder: d.Folder, CharacterID: d.CharacterID, Unattended: d.Unattended, Locale: d.Locale, Log: d.Log, Input: d.Input, Turns: d.Turns,
		expected: d.Expected, nudged: d.Nudged, printed: d.Printed, spells: d.Spells, prepared: d.Prepared, scores: d.Scores, offered: d.Offered, questions: d.Questions, classLevels: d.ClassLevels, characterRevision: d.CharacterRevision, operations: d.Operations, rules: d.Rules, chosen: d.Chosen}
	return s, nil
}

// load reads a session for its owner; anybody else's does not exist.
func (a *Agent) load(owner domain.OwnerID, id string) (*AgentSession, error) {
	rec, err := a.store.Get(a.ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.Owner != owner {
		return nil, ErrSessionNotFound()
	}
	return decode(rec)
}

func copyAgentSession(s *AgentSession) AgentSession {
	out := *s
	out.Events = append([]AgentEvent{}, s.Events...)
	for i := range out.Events {
		out.Events[i].Data = append(json.RawMessage(nil), out.Events[i].Data...)
		out.Events[i].Files = agentFileLabels(out.Events[i].Files)
		out.Events[i].Actions = append([]string(nil), out.Events[i].Actions...)
		out.Events[i].Options = append([]string(nil), out.Events[i].Options...)
	}
	out.Files = append([]AgentFile{}, s.Files...)
	for i := range out.Files {
		out.Files[i].Data = nil
	}
	out.expected = append([]string(nil), s.expected...)
	out.Manual = append([]AgentManual{}, s.Manual...)
	out.Assumptions = append([]string{}, s.Assumptions...)
	out.Log = s.Log.Clone()
	out.Input = nil
	out.operations = nil
	return out
}

func clip(text string) string {
	if len(text) > 6000 {
		return text[:6000] + "…"
	}
	return text
}

func transcriptSize(input []json.RawMessage) int {
	n := 0
	for _, v := range input {
		n += len(v)
	}
	return n
}
