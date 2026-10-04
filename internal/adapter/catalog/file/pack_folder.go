package file

import (
	"encoding/json"
	"fmt"
	"io"
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
	if err == nil && !found {
		// Repository packs may keep authored artwork beside their generated
		// pack/ folder. Adopt it into the release so exports and archives carry
		// the bytes, just as manifest-declared artwork does.
		err = loadRepositoryIcons(p, filepath.Join(folder.Path, "spell-icons"))
	}
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

func loadRepositoryIcons(p *PackDocument, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var spells []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(p.Entities["spells"], &spells); err != nil {
		if p.Entities["spells"] == nil {
			return nil
		}
		return err
	}
	var total int
	if p.Icons != nil {
		for _, data := range p.Icons.Spells {
			total += len(data)
		}
	}
	for _, spell := range spells {
		if !validLocalID(spell.Slug) {
			return fmt.Errorf("invalid spell icon identity %q", spell.Slug)
		}
		if p.Icons != nil && p.Icons.Spells[spell.Slug] != nil {
			continue
		}
		f, err := os.Open(filepath.Join(dir, spell.Slug+".webp"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, int64(maxPackBytes-total+1)))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		total += len(data)
		if total > maxPackBytes {
			return fmt.Errorf("repository spell icons exceed pack size limit")
		}
		if p.Icons == nil {
			p.Icons = &PackIcons{Spells: map[string][]byte{}}
		}
		if p.Icons.Spells == nil {
			p.Icons.Spells = map[string][]byte{}
		}
		p.Icons.Spells[spell.Slug] = data
	}
	return p.validateIcons()
}
