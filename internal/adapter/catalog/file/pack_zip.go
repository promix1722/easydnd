package file

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// ImportZIP never extracts files. The bounded archive is an isolated read-only filesystem.
func (a *Authoring) ImportZIP(data []byte) ([]byte, error) {
	if len(data) > maxPackBytes {
		return nil, fmt.Errorf("pack exceeds size limit")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(archive.File) > 4096 {
		return nil, fmt.Errorf("too many archive entries")
	}
	files := map[string]*zip.File{}
	roots := map[string]bool{}
	var total uint64
	for _, f := range archive.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || f.Mode()&fs.ModeSymlink != 0 || (!f.FileInfo().IsDir() && !f.Mode().IsRegular()) {
			return nil, fmt.Errorf("invalid archive path %q", f.Name)
		}
		if _, ok := files[name]; ok {
			return nil, fmt.Errorf("duplicate archive path %q", name)
		}
		files[name] = f
		if f.UncompressedSize64 > maxPackBytes || total > maxPackBytes-f.UncompressedSize64 {
			return nil, fmt.Errorf("expanded pack exceeds size limit")
		}
		total += f.UncompressedSize64
		if !f.FileInfo().IsDir() && (path.Base(name) == "manifest.json" || path.Base(name) == "pack-manifest.json") {
			roots[path.Dir(name)] = true
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("archive must contain exactly one pack root")
	}
	var root string
	for r := range roots {
		root = r
	}
	if root != "." && strings.Contains(root, "/") {
		return nil, fmt.Errorf("pack must be at archive root or in one enclosing folder")
	}
	consumed := 0
	read := func(name string) ([]byte, error) {
		if !fs.ValidPath(name) {
			return nil, fmt.Errorf("invalid pack path %q", name)
		}
		f, ok := files[path.Join(root, name)]
		if !ok {
			return nil, fs.ErrNotExist
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = r.Close() }()
		b, err := io.ReadAll(io.LimitReader(r, int64(maxPackBytes-consumed+1)))
		if err != nil {
			return nil, err
		}
		consumed += len(b)
		if consumed > maxPackBytes {
			return nil, fmt.Errorf("expanded pack exceeds size limit")
		}
		return b, nil
	}
	p, err := readPackDirectory(read)
	if err != nil {
		return nil, err
	}
	// Drafts may be incomplete. Publication performs the full semantic validation.
	p.Manifest.Files = nil
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if len(b) > maxPackBytes {
		return nil, fmt.Errorf("pack exceeds size limit")
	}
	return b, nil
}
