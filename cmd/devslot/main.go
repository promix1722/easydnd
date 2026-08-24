// Command devslot hands each worktree its own set of development ports.
//
// Every port the local stack binds -- the Vite dev server, the API, Postgres,
// and the compose project that carries them -- is derived from one number, the
// slot. Two worktrees running at once therefore need nothing more than two
// different numbers, and this command is what picks them: it binds each
// candidate port to see whether it is really free, remembers the answer in
// .dev-slot so a restart keeps the same public URL, and records the claim
// where the other worktrees can see it.
//
// It is a Go command rather than a few lines of shell for two reasons. The
// only honest free-port test is binding the exact address a server will use --
// connecting instead calls a bound-but-not-accepting socket free -- and `ss`
// is Linux-only, while this repo's local Postgres is documented to work on a
// laptop.
//
// Usage:
//
//	devslot claim [-count N] [-web P] [-api P] [-pg P] [-public-host H] [-public-base P]
//	devslot list  [same flags]
//	devslot show
//
// claim prints the slot number on stdout and a human summary on stderr; the
// Makefile reads the first and shows you the second.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "devslot: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: devslot claim|list|show [flags]")
	}

	command := args[0]
	l, err := parseLayout(args[1:])
	if err != nil {
		return err
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("working directory: %w", err)
	}

	switch command {
	case "claim":
		return claim(l, root)
	case "list":
		return list(l, root)
	case "show":
		return show(l, root)
	default:
		return fmt.Errorf("unknown command %q: want claim, list or show", command)
	}
}

func claim(l layout, root string) error {
	s, moved, err := l.claim(root, time.Now())
	if errors.Is(err, errNoSlots) {
		fmt.Fprintf(os.Stderr, "devslot: all %d slots are taken:\n", l.count)
		fmt.Fprint(os.Stderr, slotsTable(l, root))
		return fmt.Errorf("no free slot: stop one of the stacks above, or raise SLOT_COUNT")
	}
	if err != nil {
		return err
	}

	if moved {
		fmt.Fprintln(os.Stderr, "devslot: the slot this worktree used last is taken -- its address has changed")
	}
	fmt.Fprintf(os.Stderr, "devslot: %s\n", describe(l, s))
	fmt.Println(s.n)
	return nil
}

func list(l layout, root string) error {
	fmt.Print(slotsTable(l, root))
	return nil
}

func show(l layout, root string) error {
	n, ok := readSticky(root)
	if !ok {
		return fmt.Errorf("this worktree has no %s: run `make dev`", stickyName)
	}
	fmt.Println(n)
	return nil
}

// slotsTable renders the whole allocation as text. It returns a string rather
// than writing, so that the one caller printing to stderr and the one printing
// to stdout each say so themselves.
func slotsTable(l layout, root string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-5s %-6s %-6s %-6s %s\n", "slot", "web", "api", "pg", "held by")
	for n := range l.count {
		s := l.at(n)
		fmt.Fprintf(&b, "%-5d %-6d %-6d %-6d %s\n", s.n, s.web, s.api, s.pg, status(l, s, root))
	}
	return b.String()
}

func status(l layout, s slot, root string) string {
	path, at, ok := l.holder(s.n)
	switch {
	case !ok && l.free(s):
		return "free"
	case !ok:
		return "busy (no claim -- something else is on these ports)"
	case path == root:
		return "this worktree"
	case !l.free(s), time.Since(at) <= staleAfter:
		return shorten(path)
	default:
		return "free (stale claim from " + shorten(path) + ")"
	}
}

// shorten trims a worktree path to its last two segments, which is what tells
// two checkouts of the same project apart without wrapping the line.
func shorten(path string) string {
	parent, leaf := filepath.Split(filepath.Clean(path))
	if parent == "" {
		return leaf
	}
	return filepath.Join(filepath.Base(filepath.Clean(parent)), leaf)
}

func describe(l layout, s slot) string {
	line := fmt.Sprintf("slot %d  web %d", s.n, s.web)
	if url := l.publicURL(s); url != "" {
		line += " -> " + url
	}
	return line + fmt.Sprintf("  api %d  pg %d", s.api, s.pg)
}
