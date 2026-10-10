// Package filetest hands tests the SRD compendium the way the server loads it.
package filetest

import (
	"path/filepath"
	"runtime"
	"sync"

	file "github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

// SRDDir is the committed SRD pack, wherever the test binary is run from.
func SRDDir() string {
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "..", "..", "..", "..", "data", "pack", "srd-5.1")
}

// SRD returns one registry over the SRD pack for the whole test binary: the
// pack is 19 MB and is validated on load, which is not a cost to pay per test.
// It panics on a pack that does not load, since no test can run without it.
var SRD = sync.OnceValue(func() *file.Registry {
	r, err := file.NewRegistry([]string{SRDDir()}, nil, "")
	if err != nil {
		panic(err)
	}
	return r
})
