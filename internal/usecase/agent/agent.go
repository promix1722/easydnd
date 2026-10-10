package agent

// The import coordinator owns one isolated draft per session. External model
// calls never hold its mutex; a generation check fences their late results.
import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
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
type AgentCall struct{ ID, Name, Arguments string }

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
func (a *Agent) Close()         { a.cancel(); a.wg.Wait() }
func (a *Agent) Enabled() bool  { return a.model != nil }
func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func (a *Agent) signal() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
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

// stop cancels the turn this process has in flight for a session, if any. A
// turn running elsewhere finds out at its next write, which the store refuses.
func (a *Agent) stop(id string) {
	a.local.Lock()
	if cancel := a.running[id]; cancel != nil {
		cancel()
	}
	a.local.Unlock()
}

func (a *Agent) Create(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale, files []AgentFile, instructions string, selected ...pack.Lock) (AgentSession, error) {
	return a.create(ctx, owner, folder, locale, files, instructions, false, selected...)
}

// CreateUnattended starts a session whose owner asked not to be asked.
func (a *Agent) CreateUnattended(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale, files []AgentFile, instructions string, selected ...pack.Lock) (AgentSession, error) {
	return a.create(ctx, owner, folder, locale, files, instructions, true, selected...)
}

// Open starts a chat with nothing in it but the assistant's first question:
// which rules. There is no character yet -- that begins with the owner's
// first message -- so an opened chat that is walked away from costs a row.
func (a *Agent) Open(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale) (AgentSession, error) {
	s, err := a.opening(ctx, owner, folder, locale)
	if err != nil {
		return AgentSession{}, err
	}
	if err = a.keep(ctx, s, nil); err != nil {
		return AgentSession{}, err
	}
	return copyAgentSession(s), nil
}

// opening is a session as it is the moment the wizard is opened.
func (a *Agent) opening(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale) (*AgentSession, error) {
	if a.model == nil {
		return nil, types.NewNotImplementedError("agent is not configured").Because("agent.disabled")
	}
	folder, err := a.service.ResolveFolder(ctx, owner, folder)
	if err != nil {
		return nil, err
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return nil, err
	}
	s := &AgentSession{Created: a.service.Now(), ID: hex.EncodeToString(token[:]), Owner: owner, Folder: folder, Locale: locale, Status: "opening", Revision: 1, Files: []AgentFile{}, Events: []AgentEvent{}, Manual: []AgentManual{}, Assumptions: []string{}}
	// The assistant's first words. An event like any other, so that the page
	// draws the whole conversation one way from its first line.
	addAgentEvent(s, "opening", "", "", nil)
	return s, nil
}

// keep stores a new session, with the attachments its first message brought.
func (a *Agent) keep(ctx context.Context, s *AgentSession, files []AgentFile) error {
	rec := encode(s, nil)
	rec.Files = files
	ok, err := a.store.Create(ctx, rec, a.config.MaxSessions)
	if err == nil && !ok {
		err = types.NewValidationError("session capacity reached").Because("agent.capacity")
	}
	if err != nil {
		return err
	}
	s.count = len(s.Events)
	a.signal()
	return nil
}

// choose answers the opening question: the rules the character is built
// under. It may be answered again until the first message, which makes it
// final; the answer that stands is the last.
func (a *Agent) choose(ctx context.Context, s *AgentSession, selected pack.Lock) error {
	if s.Status != "opening" {
		return types.NewValidationError("session changed").Because("agent.changed")
	}
	var cat *catalog.Catalog
	var err error
	if !selected.IsZero() {
		if a.service.PackAccess() == nil {
			return types.NewAccessDeniedError("pack selection unavailable")
		}
		if err = a.service.PackAccess().AuthorizeLock(ctx, user.ID(s.Owner), selected, pack.Lock{}); err != nil {
			return err
		}
		cat, err = catalog.LoadLocked(ctx, a.service.Source(), s.Locale, selected)
	} else {
		cat, err = a.service.Source().Load(ctx, s.Locale)
	}
	if err != nil {
		return err
	}
	s.rules, s.chosen = cat.Lock.Clone(), true
	addAgentEvent(s, "rules", "", "", map[string]any{"packs": lockedPacks(s.rules)})
	s.Revision++
	return nil
}

// lockedPacks is a rules lock as a `rules` event carries it.
func lockedPacks(lock pack.Lock) []map[string]string {
	out := []map[string]string{}
	for _, release := range lock.Packs {
		out = append(out, map[string]string{"id": release.ID, "version": release.Version})
	}
	return out
}

// start is the owner's first message. It is where the character begins: from
// here every tool call is committed to a character that is already in the
// owner's list.
//
// It is also where the conversation's language is settled, as the one the
// owner is reading the page in now. A chat is opened by arriving at the page
// and is the chat reopened on the next visit, so the language it was opened in
// can be days old: a chat opened in English and begun after switching to
// Russian was answered in English to the end.
//
// The opening question may go unanswered. A first message is an answer too --
// "here is my sheet" -- and what it leaves unsaid is taken the way a new
// character's is: the deployment's own packs. The `rules` event then says so,
// after the message, as something the assistant decided rather than something
// the owner picked.
func (a *Agent) start(ctx context.Context, s *AgentSession, locale rules.Locale, files []AgentFile, instructions string) error {
	if s.Status != "opening" {
		return types.NewValidationError("session changed").Because("agent.changed")
	}
	if (len(files) == 0 && strings.TrimSpace(instructions) == "") || len(files) > 8 || len(instructions) > 16000 {
		return types.NewValidationError("invalid import input").Because("agent.files")
	}
	total := 0
	names := map[string]bool{}
	for _, f := range files {
		total += len(f.Data)
		if len(f.Data) == 0 || len(f.Name) > 200 || names[f.Name] {
			return types.NewValidationError("invalid attachment").Because("agent.files")
		}
		names[f.Name] = true
	}
	if total > 20<<20 {
		return types.NewValidationError("attachments too large").Because("agent.files")
	}
	s.Locale = locale
	assumed := !s.chosen
	var cat *catalog.Catalog
	var err error
	if assumed {
		if cat, err = a.service.Source().Load(ctx, s.Locale); err == nil {
			s.rules, s.chosen = cat.Lock.Clone(), true
		}
	} else {
		cat, err = catalog.LoadLocked(ctx, a.service.Source(), s.Locale, s.rules)
	}
	if err != nil {
		return err
	}
	// The same entries the builder writes for a new character: its name, then
	// the rules it is built under. The wizard's character is an ordinary one
	// from its first message, so it opens in the ordinary builder.
	e := charuc.InitEvent(charuc.NewCharacter{Name: "…"})
	e.RulesLock = s.rules.Clone()
	log := domain.Log{}
	_ = log.Append(e,
		domain.Event{Type: domain.EventChange, Source: domain.GroupIdentity, Changes: []domain.Change{{Path: "identity.ruleset", Op: domain.OpSet, Value: domain.SlugValue(rules.Slug(cat.Ruleset))}}},
		domain.Event{Type: domain.EventNote, Note: "import.session:" + s.ID})
	// One write, not Create + Commit, which leaves an empty character after
	// a failed second half.
	created, err := a.service.Repository().CreateWithLog(ctx, s.Owner, s.Folder, log)
	if err != nil {
		return err
	}
	s.CharacterID, s.Log, s.characterRevision = created.ID, created.Log.Clone(), created.Revision
	s.Files = agentFileLabels(files)
	s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": instructions + "\nCreate this single character from the description and any attached sources, using the selected rules lock. Read the build context first."}))
	addAgentEvent(s, "user", instructions, "", nil)
	s.Events[len(s.Events)-1].Files = agentFileLabels(files)
	if assumed {
		addAgentEvent(s, "rules", "", "", map[string]any{"packs": lockedPacks(s.rules), "assumed": true})
	}
	s.Revision++
	return setStatus(s, "queued")
}

// create is a chat opened, given its rules and sent its first message in one
// step: what the CLI and a client that asks its questions itself do.
func (a *Agent) create(ctx context.Context, owner domain.OwnerID, folder domain.FolderID, locale rules.Locale, files []AgentFile, instructions string, unattended bool, selected ...pack.Lock) (AgentSession, error) {
	s, err := a.opening(ctx, owner, folder, locale)
	if err != nil {
		return AgentSession{}, err
	}
	s.Unattended = unattended
	var lock pack.Lock
	if len(selected) > 0 {
		lock = selected[0]
	}
	if err = a.choose(ctx, s, lock); err == nil {
		err = a.start(ctx, s, locale, files, instructions)
	}
	if err == nil {
		err = a.keep(ctx, s, files)
	}
	if err != nil {
		// No chat, so no character either: it was only ever the chat's.
		if !s.CharacterID.IsZero() {
			_ = a.service.Delete(ctx, owner, s.CharacterID)
		}
		return AgentSession{}, err
	}
	return copyAgentSession(s), nil
}

// Rules answers an opened chat's first question.
func (a *Agent) Rules(owner domain.OwnerID, id string, revision int, selected pack.Lock) (AgentSession, error) {
	return a.change(owner, id, func(s *AgentSession) ([]AgentFile, error) {
		if s.Revision != revision {
			return nil, types.NewValidationError("session changed").Because("agent.changed")
		}
		return nil, a.choose(a.ctx, s, selected)
	})
}

// begin sends an opened chat its first message, in the language it is sent in.
func (a *Agent) begin(owner domain.OwnerID, id string, revision int, locale rules.Locale, files []AgentFile, text string) (AgentSession, error) {
	var made domain.ID
	out, err := a.change(owner, id, func(s *AgentSession) ([]AgentFile, error) {
		if s.Revision != revision {
			return nil, types.NewValidationError("session changed").Because("agent.changed")
		}
		err := a.start(a.ctx, s, locale, files, text)
		made = s.CharacterID
		return files, err
	})
	if err != nil && !made.IsZero() {
		_ = a.service.Delete(a.ctx, owner, made)
	}
	return out, err
}

// pull re-reads the character, and reports whether the player has changed it
// since the agent last did. The character is theirs and open in the builder
// the whole time; the session's log is only the agent's working copy of it.
func (a *Agent) pull(ctx context.Context, s *AgentSession) (bool, error) {
	c, err := a.service.Repository().Get(ctx, s.CharacterID)
	if err != nil {
		return false, err
	}
	if !a.made(s, c) {
		return false, types.NewNotFoundError("character not found")
	}
	if c.Revision == s.characterRevision {
		return false, nil
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return true, nil
}

// made reports whether c is a character this session may write to: its
// owner's. The id a session holds is durable, as the session is, so the
// check is ownership and nothing more.
func (a *Agent) made(s *AgentSession, c domain.Character) bool {
	return c.Owner == s.Owner
}

// push writes the working copy back, refusing if the player got there first.
func (a *Agent) push(ctx context.Context, s *AgentSession) error {
	if err := a.service.Repository().Commit(ctx, s.CharacterID, s.characterRevision, s.Log, "", nil); err != nil {
		return err
	}
	c, err := a.service.Repository().Get(ctx, s.CharacterID)
	if err != nil {
		return err
	}
	s.Log, s.characterRevision = c.Log.Clone(), c.Revision
	return nil
}

// call runs one tool against the stored character and stores what it changed.
func (a *Agent) call(ctx context.Context, s *AgentSession, name string, arguments []byte) (any, error) {
	held := s.Log
	edited, err := a.pull(ctx, s)
	if types.IsNotFound(err) {
		_ = setStatus(s, "failed")
		addAgentEvent(s, "status", "failed", "", nil)
	}
	if err != nil {
		return nil, err
	}
	if edited {
		// The owner's numbers win over the sheet's -- when they changed a
		// number. A rename is no reason to forget the printed totals: without
		// them the next race or improvement lands on top of bases that were
		// solved to already include it.
		if cat, catErr := a.Catalog(ctx, *s); catErr != nil || !sameScores(held, s.Log, cat) {
			s.scores = nil
		}
		addAgentEvent(s, "edit", "", "", nil)
		if name != "get_build_context" {
			return nil, fmt.Errorf("not run: the player edited the character in the builder. Call get_build_context, treat what it shows as authoritative and preserve their changes, then send this again if it is still needed")
		}
	}
	before := s.Log.Clone()
	result, err := a.tool(ctx, s, name, arguments)
	if cat, catErr := a.Catalog(ctx, *s); catErr == nil {
		a.settleScores(s, cat)
	}
	if !reflect.DeepEqual(before, s.Log) {
		if pushErr := a.push(ctx, s); pushErr != nil {
			s.Log = before
			return nil, pushErr
		}
	}
	return result, err
}
func (a *Agent) Get(owner domain.OwnerID, id string) (AgentSession, error) {
	s, err := a.load(owner, id)
	if err != nil {
		return AgentSession{}, err
	}
	return copyAgentSession(s), nil
}

// Poll is Get for a reader that already holds the session at (revision,
// after), after being the id of its last event. A moved revision returns the
// whole session (full); otherwise only the events past after, which is what
// keeps a streaming turn's answers small; changed=false when there is nothing
// the reader lacks. It never waits: the page asks again in a second. See
// docs/polling.md.
func (a *Agent) Poll(ctx context.Context, owner domain.OwnerID, id string, revision, after int) (s AgentSession, full, changed bool, err error) {
	rec, err := a.store.Tail(ctx, id, after)
	if err == nil && rec.Owner != owner {
		err = ErrSessionNotFound()
	}
	if err != nil {
		return AgentSession{}, false, false, err
	}
	if rec.Revision != revision || after < 0 || after > rec.Count {
		s, err = a.Get(owner, id)
		return s, true, true, err
	}
	return AgentSession{Events: rec.Events}, false, after < rec.Count, nil
}
func (a *Agent) List(owner domain.OwnerID) []AgentSession {
	out := []AgentSession{}
	records, err := a.store.List(a.ctx, owner)
	if err != nil {
		a.service.Logger().Warn("AI wizard list failed", "error", err)
		return out
	}
	for _, rec := range records {
		s, err := decode(rec)
		if err != nil {
			continue
		}
		// The wizard reopens the latest unfinished chat, so one whose
		// character has since been deleted is left out.
		if c, err := a.service.Repository().Get(a.ctx, s.CharacterID); s.Status != "opening" && (err != nil || !a.made(s, c)) {
			continue
		}
		v := copyAgentSession(s)
		v.Events, v.Files = nil, nil
		out = append(out, v)
	}
	return out
}

// invalidate fences whatever turn is in flight: its writes carry the
// generation it claimed and the store refuses them from here on.
func (a *Agent) invalidate(s *AgentSession) {
	s.Generation++
	s.Revision++
}

// change applies one owner's edit to a stored session under its row lock.
func (a *Agent) change(owner domain.OwnerID, id string, edit func(*AgentSession) ([]AgentFile, error)) (AgentSession, error) {
	var out AgentSession
	err := a.store.Update(a.ctx, id, func(rec *Record) error {
		if rec.Owner != owner {
			return ErrSessionNotFound()
		}
		s, err := decode(*rec)
		if err != nil {
			return err
		}
		files, err := edit(s)
		if err != nil {
			return err
		}
		*rec = encode(s, nil)
		rec.Files = files
		out = copyAgentSession(s)
		return nil
	})
	if err != nil {
		return AgentSession{}, err
	}
	a.stop(id)
	a.signal()
	return out, nil
}
func (a *Agent) Control(owner domain.OwnerID, id, action, text string, revision int) (AgentSession, error) {
	if action == "discard" {
		s, err := a.load(owner, id)
		if err != nil {
			return AgentSession{}, err
		}
		if s.Finished || s.Revision != revision {
			return AgentSession{}, types.NewValidationError("session changed").Because("agent.changed")
		}
		// Discarding the chat discards what it made -- and only that: after a
		// restart the id it holds may be another character's. Leaving the chat
		// any other way leaves the character where it is.
		if c, err := a.service.Repository().Get(a.ctx, s.CharacterID); err == nil && a.made(s, c) {
			if err := a.service.Delete(a.ctx, owner, s.CharacterID); err != nil && !types.IsNotFound(err) {
				return AgentSession{}, err
			}
		}
		if err := a.store.Delete(a.ctx, id); err != nil {
			return AgentSession{}, err
		}
		a.stop(id)
		return copyAgentSession(s), nil
	}
	if action == "message" {
		// By this door the chat keeps the language it was opened in: a control
		// carries none. The page sends its first message with AddFiles.
		if s, err := a.load(owner, id); err == nil && s.Status == "opening" {
			return a.begin(owner, id, revision, s.Locale, nil, text)
		}
	}
	return a.change(owner, id, func(s *AgentSession) ([]AgentFile, error) {
		// An opened chat has nothing to stop, resume or finish: it is waiting
		// for its rules and its first message, which come by other doors.
		if s.Status == "opening" {
			return nil, types.NewValidationError("session changed").Because("agent.changed")
		}
		// Finishing closes the chat whatever it was doing and whatever revision
		// the page last saw: it changes nothing a stale view could be wrong about.
		if action == "finish" {
			a.invalidate(s)
			s.Finished = true
			// The import carried everything and wore nothing; this is the end,
			// so the character is dressed now, one item to a slot. Best effort:
			// a chat that made no character, or one whose character is gone,
			// still closes.
			if s.CharacterID != "" {
				_ = a.service.AutoEquip(a.ctx, s.Owner, s.CharacterID, s.Locale)
			}
			if s.Status == "queued" || s.Status == "running" {
				return nil, setStatus(s, "paused")
			}
			return nil, nil
		}
		// Stopping a turn in flight does not wait on the revision either: a
		// running turn moves it with every tool call, so the page pressing
		// Stop is a step behind more often than not, and a stop that answered
		// "session changed" would be a button that works by luck. A press that
		// arrives after the turn ended has nothing to stop and says nothing.
		if action == "stop" && !s.Finished && s.Revision != revision {
			if s.Status != "queued" && s.Status != "running" {
				return nil, nil
			}
			revision = s.Revision
		}
		if s.Finished || s.Revision != revision {
			return nil, types.NewValidationError("session changed").Because("agent.changed")
		}
		switch action {
		case "stop":
			a.invalidate(s)
			// A stop is accepted in any state, a paused one included.
			s.Status = "paused"
			addAgentEvent(s, "status", "paused", "", nil)
		case "resume", "retry":
			if len(s.Input) > 500 || transcriptSize(s.Input) > 2<<20 {
				return nil, types.NewValidationError("session conversation limit reached").Because("agent.limit")
			}
			a.invalidate(s)
			s.Turns = 0
			return nil, setStatus(s, "queued")
		case "message":
			if strings.TrimSpace(text) == "" || len(text) > 16000 || len(s.Input) > 500 || transcriptSize(s.Input) > 2<<20 {
				return nil, types.NewValidationError("invalid message")
			}
			a.invalidate(s)
			s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": text}))
			addAgentEvent(s, "user", text, "", nil)
			s.replied(text)
			s.Turns = 0
			return nil, setStatus(s, "queued")
		default:
			return nil, types.NewValidationError("unknown session control")
		}
		return nil, nil
	})
}
func (a *Agent) Catalog(ctx context.Context, s AgentSession) (*catalog.Catalog, error) {
	cat, err := catalog.LoadLocked(ctx, a.service.Source(), s.Locale, s.Log.RulesLock())
	return domain.WithCustomCatalog(s.Log, cat), err
}
func (a *Agent) Sheet(ctx context.Context, s AgentSession) (domain.State, error) {
	cat, err := a.Catalog(ctx, s)
	if err != nil {
		return domain.State{}, err
	}
	return domain.Project(s.Log, cat)
}
func (a *Agent) worker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.wake:
		}
		for a.ctx.Err() == nil {
			// A turn this process is still unwinding is not taken again: its
			// late tool call must not run beside the next turn's.
			a.local.Lock()
			busy := slices.Collect(maps.Keys(a.running))
			a.local.Unlock()
			rec, ok, err := a.store.Claim(a.ctx, a.instance, a.config.Timeout+leaseMargin, busy)
			if err != nil && a.ctx.Err() == nil {
				a.service.Logger().Warn("AI wizard claim failed", "error", err)
			}
			if err != nil || !ok {
				break
			}
			ctx, cancel := context.WithTimeout(a.ctx, a.config.Timeout)
			a.local.Lock()
			a.running[rec.ID] = cancel
			a.local.Unlock()
			a.signal()
			a.turn(ctx, cancel, rec)
			cancel()
			a.local.Lock()
			delete(a.running, rec.ID)
			a.local.Unlock()
			a.signal()
		}
	}
}

// turn is one claimed turn. The session it works on is this goroutine's own:
// the lease is what keeps every other writer out, so nothing here is locked.
func (a *Agent) turn(ctx context.Context, cancel context.CancelFunc, rec Record) {
	s, err := decode(rec)
	if err == nil {
		s.Files, err = a.store.Files(ctx, s.ID)
	}
	if err != nil {
		a.service.Logger().Error("AI wizard session unreadable", "session", rec.ID, "error", err)
		return
	}
	// save stores the session as it stands and reports whether the turn is
	// still this process's to continue.
	save := func(pending []AgentCall, last bool) bool {
		// A turn that ran out of time, or a process that is stopping, still
		// has to say how it ended.
		saving, done := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
		defer done()
		// ponytail: the whole document is rewritten after every tool call (up
		// to 2 MiB of transcript); store Input as rows if it shows.
		rec := encode(s, pending)
		if !last {
			// A question or a review ends the turn in the middle of a batch,
			// and the status goes with the turn's last save, not this one: a
			// session stored as no longer running has given up its lease, and
			// the last save -- the one that moves the revision the page is
			// waiting on -- would be refused.
			rec.Status = "running"
		}
		ok, err := a.store.Save(saving, rec, a.instance)
		if err != nil {
			a.service.Logger().Error("AI wizard save failed", "session", s.ID, "error", err)
		}
		if err != nil || !ok {
			cancel()
			return false
		}
		s.count = len(s.Events)
		return true
	}
	// Before the model is asked anything: a chat whose character has been
	// deleted has nothing to build, and a request would be spent finding
	// that out.
	if c, err := a.service.Repository().Get(ctx, s.CharacterID); err != nil && ctx.Err() == nil || err == nil && !a.made(s, c) {
		a.service.Logger().Warn("AI wizard session has no character of its own", "session", s.ID, "character", s.CharacterID, "error", err)
		_ = setStatus(s, "failed")
		s.Revision++
		addAgentEvent(s, "status", "failed", "", nil)
		save(nil, true)
		return
	}
	held := true
	response, err := a.model.Respond(ctx, AgentRequest{Input: s.Input, Files: s.Files, Locale: s.Locale.String(), Session: s.ID, Unattended: s.Unattended}, func(delta string) {
		if !held || len(s.Events) >= 10000 {
			return
		}
		e := AgentEvent{ID: len(s.Events) + 1, Kind: "delta", Text: delta}
		if ok, err := a.store.Append(ctx, s.ID, a.instance, s.Generation, e); err != nil || !ok {
			// Stopped, answered or taken over while the model was typing.
			held = false
			cancel()
			return
		}
		s.Events = append(s.Events, e)
		s.count = len(s.Events)
	})
	if !held {
		return
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		if a.ctx.Err() != nil {
			// The process is stopping. The turn is not lost with it: back in
			// the queue, it is the next process's, or this one's restarted.
			_ = setStatus(s, "queued")
		} else {
			// The owner is told only that it failed; why is here.
			a.service.Logger().Error("AI wizard model request failed", "session", s.ID, "error", err)
			_ = setStatus(s, "failed")
			s.Revision++
			addAgentEvent(s, "status", "failed", "", nil)
		}
		save(nil, true)
		return
	}
	// Commit a complete response before executing any tool. Partial streamed
	// arguments are never interpreted. The transcript is the operation ledger.
	s.Input = append(s.Input, response.Output...)
	a.service.Logger().Info("AI wizard model request", "session", s.ID, "inputTokens", response.Usage.Input, "cachedTokens", response.Usage.Cached, "outputTokens", response.Usage.Output)
	addAgentEvent(s, "response", response.Text, "", nil)
	s.Turns++
	if s.operations == nil {
		s.operations = map[string]agentOperation{}
	}
	for i, call := range response.Calls {
		hash := sha256.Sum256([]byte(call.Name + "\x00" + call.Arguments))
		op, known := s.operations[call.ID]
		if known && op.Hash != hash {
			op = agentOperation{Hash: hash, Error: true, Result: raw(map[string]string{"error": "operation id reused with different arguments"})}
		} else if !known {
			var result any
			// Every call still gets an output -- the provider rejects a
			// transcript with an unanswered call -- but a response is one
			// bounded batch, and nothing runs after the call that ended the turn.
			switch {
			case ctx.Err() != nil:
				err = errInterrupted
			case i >= 32:
				err = fmt.Errorf("too many tool calls in one response; send the rest again")
			case s.Status != "running":
				err = fmt.Errorf("not run: the turn had already ended with a question or a review; send it again if it is still needed")
			case len(call.Arguments) > 128<<10:
				err = fmt.Errorf("tool arguments too large")
			default:
				result, err = a.call(ctx, s, call.Name, []byte(call.Arguments))
			}
			op = agentOperation{Hash: hash, Result: raw(result)}
			if err != nil {
				a.service.Logger().Warn("AI wizard tool rejected", "tool", call.Name, "error", err)
				op.Error = true
				op.Result = agentError(err)
			}
			// The only record of what a model actually asked for. Debug, because
			// the arguments are a player's character sheet.
			a.service.Logger().Debug("AI wizard tool call", "session", s.ID, "tool", call.Name, "arguments", clip(call.Arguments), "result", clip(string(op.Result)))
			s.operations[call.ID] = op
		}
		s.Input = append(s.Input, raw(map[string]any{"type": "function_call_output", "call_id": call.ID, "output": string(op.Result)}))
		var args agentArgs
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		summary := args.Query
		if summary == "" {
			summary = args.Path
		}
		if summary == "" {
			summary = args.Ref
		}
		if summary == "" {
			summary = args.Name
		}
		if op.Error {
			summary = "invalid"
		}
		addAgentEvent(s, "tool", summary, call.Name, nil)
		if op.Error {
			s.Events[len(s.Events)-1].Data = op.Result
		}
		// Batches and answers publish their own progress, one line per write.
		if tool := agentToolName(call.Name, args); !op.Error && !known && (tool == "resolve_import_facts" || tool == "upsert_custom_option" || tool == "revise_choice") {
			a.recordProgress(ctx, s, tool, args, op.Result)
		}
		s.Events[len(s.Events)-1].Source = args.Source
		s.Events[len(s.Events)-1].Assumption = args.Assumption
		// Each call's writes are stored, and so shown, as it finishes, not
		// when the batch does.
		if !save(response.Calls[i+1:], false) {
			return
		}
	}

	s.Revision++
	if s.Status == "running" {
		if len(response.Calls) == 0 {
			// The request demands a tool call, so this is a response cut short.
			// It is not a question: waiting would leave the owner with nothing
			// to answer. Paused has a Resume button.
			_ = setStatus(s, "paused")
			addAgentEvent(s, "status", "paused", "", nil)
		} else if s.Turns >= a.config.MaxTurns || len(s.Input) > 500 || len(s.Events) > 10000 || transcriptSize(s.Input) > 2<<20 {
			_ = setStatus(s, "paused")
			addAgentEvent(s, "status", "budget", "", nil)
		} else {
			_ = setStatus(s, "queued")
		}
	}
	save(nil, true)
}

func clip(text string) string {
	if len(text) > 6000 {
		return text[:6000] + "…"
	}
	return text
}

// agentError is what a rejected call tells the model: the message, and
// whatever would make its next attempt different -- the fields a validator
// named, the entries a name could have meant. A bare "some answers are not
// valid" is how a model ends up preserving the content as custom instead.
func agentError(err error) json.RawMessage {
	out := map[string]any{"error": err.Error()}
	var invalid *types.FieldValidationError
	if errors.As(err, &invalid) {
		fields := []map[string]any{}
		for _, f := range invalid.Fields {
			fields = append(fields, map[string]any{"field": f.Field, "rule": f.Rule, "reason": f.Reason, "args": f.Args})
		}
		out["fields"] = fields
	}
	var unsettled *candidatesError
	if errors.As(err, &unsettled) {
		out["candidates"] = unsettled.Candidates
	}
	return raw(out)
}

func transcriptSize(input []json.RawMessage) int {
	n := 0
	for _, v := range input {
		n += len(v)
	}
	return n
}

// AddFiles extends the same character's source set and fences an in-flight
// response, just like a correction. Files remain private to this session.
//
// To an opened chat it is the first message, and locale is the language the
// conversation is then held in; later it changes nothing.
func (a *Agent) AddFiles(owner domain.OwnerID, id string, revision int, locale rules.Locale, files []AgentFile, text string) (AgentSession, error) {
	if s, err := a.load(owner, id); err == nil && s.Status == "opening" {
		return a.begin(owner, id, revision, locale, files, text)
	}
	stored, err := a.store.Files(a.ctx, id)
	if err != nil {
		return AgentSession{}, err
	}
	return a.change(owner, id, func(s *AgentSession) ([]AgentFile, error) {
		if s.Revision != revision {
			return nil, types.NewValidationError("session changed").Because("agent.changed")
		}
		if len(files) == 0 || len(files)+len(stored) > 8 || len(text) > 16000 || len(s.Input) > 500 {
			return nil, types.NewValidationError("invalid attachments").Because("agent.files")
		}
		total := 0
		names := map[string]bool{}
		for _, f := range stored {
			total += len(f.Data)
			names[f.Name] = true
		}
		for _, f := range files {
			total += len(f.Data)
			if len(f.Data) == 0 || len(f.Name) > 200 || names[f.Name] {
				return nil, types.NewValidationError("empty or duplicate attachment").Because("agent.files")
			}
			names[f.Name] = true
		}
		if total > 20<<20 {
			return nil, types.NewValidationError("attachments too large").Because("agent.files")
		}
		s.Files = append(s.Files, agentFileLabels(files)...)
		a.invalidate(s)
		s.Turns = 0
		s.Input = append(s.Input, raw(map[string]any{"role": "user", "content": "I added more source files for the same character. Read the full source set and preserve my edits.\n" + text}))
		addAgentEvent(s, "user", text, "", nil)
		s.Events[len(s.Events)-1].Files = agentFileLabels(files)
		if strings.TrimSpace(text) != "" {
			s.replied(text)
		}
		return files, setStatus(s, "queued")
	})
}

// Attachments belong to the user message which submitted them, never to a
// separate transcript row. Source bytes remain private to the provider input.
func agentFileLabels(files []AgentFile) []AgentFile {
	out := make([]AgentFile, len(files))
	for i, f := range files {
		out[i] = AgentFile{Name: f.Name, MIME: f.MIME}
	}
	return out
}
