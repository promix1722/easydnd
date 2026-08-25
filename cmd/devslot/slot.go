package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// stickyName is the file, in the worktree root, that remembers which slot this
// worktree claimed. It is what makes the public URL survive a restart: without
// it every `make dev` would take the lowest free slot and the address you
// bookmarked would belong to whoever started first.
const stickyName = ".dev-slot"

// staleAfter is how long a claim survives with none of its ports listening.
//
// A claim is written by this command and the servers it precedes take a moment
// to bind, so "no listener" cannot mean "dead" immediately or two worktrees
// starting seconds apart would pick the same slot. Once a stack is up its
// ports are the evidence and this grace stops mattering; once it is stopped
// the claim ages out and the slot returns to the pool.
const staleAfter = time.Minute

// layout is the whole port plan: how many slots exist and what each one binds.
// The bases arrive as flags rather than constants so the Makefile stays the
// single place the numbers are written down.
type layout struct {
	count      int
	webBase    int
	apiBase    int
	pgBase     int
	publicHost string
	publicBase int

	// probe reports whether a loopback TCP port can be bound. A field so tests
	// can say what is busy without racing a real listener.
	probe func(port int) bool

	// claims is the directory holding one file per claimed slot, shared by
	// every worktree on the machine.
	claims string
}

// slot is one allocation: a number and the three ports derived from it.
type slot struct {
	n   int
	web int
	api int
	pg  int
}

func (l layout) at(n int) slot {
	return slot{n: n, web: l.webBase + n, api: l.apiBase + n, pg: l.pgBase + n}
}

func (s slot) ports() [3]int { return [3]int{s.web, s.api, s.pg} }

// publicURL is where a browser reaches this slot's web server, or "" when the
// machine has no proxy in front (a laptop, where the dev server is the origin).
func (l layout) publicURL(s slot) string {
	if l.publicHost == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", l.publicHost, l.publicBase+s.n)
}

// free reports whether every port this slot needs can be bound right now.
func (l layout) free(s slot) bool {
	for _, p := range s.ports() {
		if !l.probe(p) {
			return false
		}
	}
	return true
}

// portFree binds and immediately releases the exact address a server will use.
// Connecting to the port instead would answer a different question: a socket
// bound but not accepting refuses the connection and would look free.
func portFree(port int) bool {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func (l layout) claimPath(n int) string {
	return filepath.Join(l.claims, strconv.Itoa(n))
}

// holder returns the worktree that claimed slot n, and when it said so.
func (l layout) holder(n int) (path string, at time.Time, ok bool) {
	info, err := os.Stat(l.claimPath(n))
	if err != nil {
		return "", time.Time{}, false
	}
	b, err := os.ReadFile(l.claimPath(n))
	if err != nil {
		return "", time.Time{}, false
	}
	return strings.TrimSpace(string(b)), info.ModTime(), true
}

// ours reports that root is the worktree recorded against this slot.
func (l layout) ours(s slot, root string) bool {
	path, _, ok := l.holder(s.n)
	return ok && path == root
}

// available reports whether root may take this slot: the ports are free, and
// either nobody claims it, we claim it already, or the claim has aged out with
// nothing listening behind it.
func (l layout) available(s slot, root string, now time.Time) bool {
	if !l.free(s) {
		return false
	}
	path, at, ok := l.holder(s.n)
	if !ok || path == root {
		return true
	}
	return now.Sub(at) > staleAfter
}

func (l layout) take(s slot, root string) error {
	if err := os.MkdirAll(l.claims, 0o700); err != nil {
		return fmt.Errorf("create claim directory: %w", err)
	}
	if err := os.WriteFile(l.claimPath(s.n), []byte(root+"\n"), 0o600); err != nil {
		return fmt.Errorf("write claim: %w", err)
	}
	return writeSticky(root, s.n)
}

// errNoSlots is returned when every slot is spoken for. The caller turns it
// into a listing, because "no free slot" without saying who holds them is not
// an error anybody can act on.
var errNoSlots = errors.New("no free slot")

// claim resolves this worktree's slot, preferring the one it used last.
// moved reports that the sticky slot was taken and the URL has changed.
func (l layout) claim(root string, now time.Time) (s slot, moved bool, err error) {
	// A slot this worktree already holds is kept even when its ports are busy,
	// because the thing holding them is almost always this worktree's own
	// stack -- a `make db/up` from earlier, or a `make dev` still running.
	// Moving would start a second stack at a second address and orphan the
	// first; failing to bind says so instead. Only a slot another worktree has
	// taken makes us move, which is exactly what the message below reports.
	if n, ok := readSticky(root); ok && n >= 0 && n < l.count {
		if kept := l.at(n); l.ours(kept, root) || l.available(kept, root, now) {
			return kept, false, l.take(kept, root)
		}
		moved = true
	}

	for n := range l.count {
		if candidate := l.at(n); l.available(candidate, root, now) {
			return candidate, moved, l.take(candidate, root)
		}
	}
	return slot{}, moved, errNoSlots
}

func readSticky(root string) (int, bool) {
	b, err := os.ReadFile(filepath.Join(root, stickyName))
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return n, true
}

func writeSticky(root string, n int) error {
	if err := os.WriteFile(filepath.Join(root, stickyName), []byte(strconv.Itoa(n)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", stickyName, err)
	}
	return nil
}
