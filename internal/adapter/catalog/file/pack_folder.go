package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// PackFolder installs one pack from a directory or a repository's pack/ folder.
// ID optionally gives a replacement core its own namespace alongside the SRD.
type PackFolder struct {
	Path string
	ID   string
}

func loadPackFolder(folder PackFolder) (*PackDocument, error) {
	info, err := os.Stat(folder.Path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("expected a pack directory")
	}
	path := folder.Path
	found := false
	for _, name := range []string{"pack-manifest.json", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			found = true
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if !found {
		path = filepath.Join(path, "pack")
	}
	p, err := LoadPack(path)
	if err != nil || folder.ID == "" || folder.ID == p.Manifest.ID {
		return p, err
	}
	b, err := EncodePack(p)
	if err != nil {
		return nil, err
	}
	b, err = rewritePackIdentity(b, folder.ID, nil, false)
	if err != nil {
		return nil, err
	}
	return DecodePack(b)
}
