// Package pack defines immutable release identities and reproducible rules locks.
// Decoding, hashing, dependency solving and storage belong to adapters.
package pack

import (
	"encoding/hex"
	"fmt"
	"slices"
)

const Semantics = "1"
const BaseID = "srd-2014"

type Release struct {
	ID, Version, Digest string
}

// Lock includes the complete dependency closure, sorted by pack ID.
type Lock struct {
	Edition, Semantics string
	Packs              []Release
}

func (l Lock) IsZero() bool { return len(l.Packs) == 0 }
func (l Lock) Clone() Lock  { l.Packs = slices.Clone(l.Packs); return l }
func (l Lock) Equal(other Lock) bool {
	return l.Edition == other.Edition && l.Semantics == other.Semantics && slices.Equal(l.Packs, other.Packs)
}
func (l Lock) Validate() error {
	if l.IsZero() {
		return fmt.Errorf("empty rules lock")
	}
	if l.Edition == "" || l.Semantics != Semantics {
		return fmt.Errorf("unsupported rules semantics %q or missing edition", l.Semantics)
	}
	last := ""
	for _, p := range l.Packs {
		if p.ID <= last || p.Version == "" || len(p.Digest) != 64 {
			return fmt.Errorf("invalid or unsorted locked release %q", p.ID)
		}
		if _, err := hex.DecodeString(p.Digest); err != nil {
			return fmt.Errorf("invalid release digest")
		}
		last = p.ID
	}
	return nil
}
