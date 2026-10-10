package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
)

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
// room refuses a chat's first message once its owner has started as many
// chats in the last day as they may, or holds as many characters: a started
// chat is a character.
//
// ponytail: counted from the chats still stored, so discarding one gives its
// run back, and a chat opened more than a day before it was started is not
// counted. A table of (owner, started_at) closes both.
//
// It reads the store, so it runs before start and never inside it: begin
// calls start under the chat's own lock.
func (a *Agent) room(ctx context.Context, owner domain.OwnerID, id string) error {
	limits := a.service.Limits()
	records, err := a.store.List(ctx, owner)
	if err != nil {
		return err
	}
	since, runs := a.service.Now().Add(-24*time.Hour), 0
	for _, rec := range records {
		if rec.ID != id && rec.Status != "opening" && rec.Created.After(since) {
			runs++
		}
	}
	if runs >= limits.WizardRunsPerDay {
		return types.LimitReached("wizardRuns", limits.WizardRunsPerDay)
	}
	return a.service.CheckCharacterLimit(ctx, owner)
}

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
	if err = a.room(ctx, owner, s.ID); err == nil {
		err = a.choose(ctx, s, lock)
	}
	if err == nil {
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
	if err := a.room(a.ctx, owner, id); err != nil {
		return AgentSession{}, err
	}
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
