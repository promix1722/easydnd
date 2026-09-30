package helpers

import "github.com/promix1722/easydnd/internal/domain/pack"

type PackRelease struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}
type RulesLock struct {
	Edition   string        `json:"edition"`
	Semantics string        `json:"semantics"`
	Packs     []PackRelease `json:"packs"`
}

func RulesLockOf(l pack.Lock) RulesLock {
	out := RulesLock{Edition: l.Edition, Semantics: l.Semantics}
	for _, r := range l.Packs {
		out.Packs = append(out.Packs, PackRelease{r.ID, r.Version, r.Digest})
	}
	return out
}
func (l RulesLock) Domain() pack.Lock {
	out := pack.Lock{Edition: l.Edition, Semantics: l.Semantics}
	for _, r := range l.Packs {
		out.Packs = append(out.Packs, pack.Release{ID: r.ID, Version: r.Version, Digest: r.Digest})
	}
	return out
}
