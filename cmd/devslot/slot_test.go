package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testLayout is the real port plan with a fake prober: every port is free
// unless the test says otherwise, and claims land in a temporary directory
// rather than the machine's.
func testLayout(t *testing.T, busy ...int) layout {
	t.Helper()
	taken := make(map[int]bool, len(busy))
	for _, p := range busy {
		taken[p] = true
	}
	return layout{
		count:      3,
		webBase:    8080,
		apiBase:    18080,
		pgBase:     5440,
		publicHost: "example.test",
		publicBase: 8880,
		probe:      func(port int) bool { return !taken[port] },
		claims:     t.TempDir(),
	}
}

func TestPortFreeSeesABoundPort(t *testing.T) {
	// The one test that exercises the real prober. Everything else fakes it,
	// because a test that races a live listener is a test that fails in CI for
	// reasons that have nothing to do with this package.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if portFree(port) {
		t.Errorf("portFree(%d) = true while a listener holds it", port)
	}
}

func TestClaimSkipsASlotWhoseAnyPortIsBusy(t *testing.T) {
	// Slot 0's Postgres port is taken -- by a neighbour's container, say. The
	// web and API ports of that slot are free, which is exactly the case a
	// one-port check would get wrong.
	l := testLayout(t, 5440)
	root := t.TempDir()

	s, moved, err := l.claim(root, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 1 {
		t.Errorf("slot = %d, want 1 (slot 0 has a busy pg port)", s.n)
	}
	if moved {
		t.Error("moved = true for a worktree that had no slot to move from")
	}
	if s.web != 8081 || s.api != 18081 || s.pg != 5441 {
		t.Errorf("ports = %d/%d/%d, want 8081/18081/5441", s.web, s.api, s.pg)
	}
}

func TestClaimKeepsTheSlotThisWorktreeUsedLast(t *testing.T) {
	// The address you bookmarked has to survive a restart, so a free sticky
	// slot wins over the lower-numbered one that is also free.
	l := testLayout(t)
	root := t.TempDir()
	seedSticky(t, root, 2)

	s, moved, err := l.claim(root, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 2 {
		t.Errorf("slot = %d, want the sticky 2", s.n)
	}
	if moved {
		t.Error("moved = true although the sticky slot was kept")
	}
}

func TestClaimMovesOnWhenTheStickySlotIsTaken(t *testing.T) {
	l := testLayout(t, 8080)
	root := t.TempDir()
	seedSticky(t, root, 0)

	s, moved, err := l.claim(root, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 1 {
		t.Errorf("slot = %d, want 1", s.n)
	}
	if !moved {
		t.Error("moved = false; the caller has to be told the address changed")
	}
	if n, ok := readSticky(root); !ok || n != 1 {
		t.Errorf("sticky file = %d (%v), want 1", n, ok)
	}
}

func TestClaimRefusesASlotAnotherWorktreeJustTook(t *testing.T) {
	// Ports are free because that worktree's servers have not bound yet. The
	// claim file is the only thing that stops both of them taking slot 0.
	l := testLayout(t)
	neighbour := t.TempDir()
	if _, _, err := l.claim(neighbour, time.Now()); err != nil {
		t.Fatalf("neighbour claim: %v", err)
	}

	root := t.TempDir()
	s, _, err := l.claim(root, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 1 {
		t.Errorf("slot = %d, want 1 -- slot 0 was claimed seconds ago", s.n)
	}
}

func TestClaimReusesASlotWhoseClaimWentStale(t *testing.T) {
	// Same situation, an hour later: nothing is listening and the claim has
	// aged out, so the slot is back in the pool.
	l := testLayout(t)
	neighbour := t.TempDir()
	if _, _, err := l.claim(neighbour, time.Now()); err != nil {
		t.Fatalf("neighbour claim: %v", err)
	}

	root := t.TempDir()
	s, _, err := l.claim(root, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 0 {
		t.Errorf("slot = %d, want 0 -- the claim on it is stale", s.n)
	}
}

func TestClaimReportsEveryHolderWhenNothingIsFree(t *testing.T) {
	l := testLayout(t, 8080, 8081, 8082)
	root := t.TempDir()

	if _, _, err := l.claim(root, time.Now()); err == nil {
		t.Fatal("claim succeeded with every slot busy")
	} else if !strings.Contains(err.Error(), "no free slot") {
		t.Errorf("err = %v, want it to say there is no free slot", err)
	}
}

func TestPublicURLIsEmptyWithoutAProxy(t *testing.T) {
	// A laptop with nothing in front: the dev server is the origin, and
	// inventing a public URL for it would put a wrong entry in rp_origins.
	l := testLayout(t)
	l.publicHost = ""
	if got := l.publicURL(l.at(1)); got != "" {
		t.Errorf("publicURL = %q, want empty", got)
	}
	l.publicHost = "example.test"
	if got, want := l.publicURL(l.at(1)), "http://example.test:8881"; got != want {
		t.Errorf("publicURL = %q, want %q", got, want)
	}
}

func seedSticky(t *testing.T, root string, n int) {
	t.Helper()
	path := filepath.Join(root, stickyName)
	if err := os.WriteFile(path, []byte(strconv.Itoa(n)+"\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestClaimKeepsOurOwnSlotEvenWhenItsPortsAreBusy(t *testing.T) {
	// The busy port is this worktree's own Postgres, still up from an earlier
	// `make db/up`. Moving would start a second stack at a second address and
	// leave the first orphaned; keeping the slot lets the bind fail and say so.
	l := testLayout(t)
	root := t.TempDir()
	if _, _, err := l.claim(root, time.Now()); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	busy := testLayout(t, 5440)
	busy.claims = l.claims

	s, moved, err := busy.claim(root, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.n != 0 {
		t.Errorf("slot = %d, want the 0 this worktree already holds", s.n)
	}
	if moved {
		t.Error("moved = true although the slot was kept")
	}
}
