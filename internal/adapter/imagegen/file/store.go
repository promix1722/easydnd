// Package file keeps generated spell icons as seed packs on disk.
//
// Every pack is a flat directory of <key>.webp files named by the local slug
// they answer to; the key's namespace is the directory it lives in, not a
// part of the file name. The base directory outputDir holds the SRD pack --
// bare slugs like "acid-arrow". Each entry of packDirs maps one catalogue
// pack id to the directory that owns its icons, wherever that repository
// lives: "dnd-2014/acid-arrow" is <packDirs["dnd-2014"]>/acid-arrow.webp.
// Mapping is operator configuration alone; a namespaced slug with no
// configured pack is refused before any paid request rather than written
// into the wrong repository.
//
// A namespaced slug with no icon of its own falls back to the flat SRD file
// of its last segment, so legacy base art still serves pack-local copies of
// the same spell.
//
// Generated PNG masters are written to a separate cache directory, retained
// as the original artwork for manual reroll and reference: nothing in the
// service reads them back, and a replace run always asks the provider for
// new art. The webp is what gets seeded into the database. Every write goes
// through a sibling temp file and a rename, so a failed save leaves the
// previous icon, or no icon, but never a truncated one.
package file

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"

	"github.com/promix1722/easydnd/internal/domain/imageasset"
	"github.com/promix1722/easydnd/internal/types"
	"github.com/promix1722/easydnd/internal/usecase/spellicon"
)

// IconSize is the published edge; the 1024px masters are downscaled to it.
const IconSize = 128

// contentType is what every stored icon is.
const contentType = "image/webp"

// maxImageBytes bounds a read; a seed icon is tens of kilobytes, and eight
// megabytes already means something that is not an icon was dropped in the
// pack.
const maxImageBytes = 8 << 20

// slugSegment is one path segment of a slug: alphanumeric with '-' and '_'
// inside, which rules out '.', '..', slashes and everything else a path can
// smuggle. Two segments are the most a catalogue slug ever has
// ("pack/local").
var (
	slugSegment = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)
	slugRE      = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?(/[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?)?$`)
)

// Store is the disk seed packs and PNG cache.
type Store struct {
	outputDir string
	cacheDir  string
	// packDirs maps a catalogue pack id to the directory that owns its
	// icons; a namespaced slug whose namespace is absent here is refused.
	packDirs map[string]string
}

// New builds the store. packDirs names each non-SRD pack's directory by its
// catalogue id and is copied, so later edits to the caller's map cannot
// reroute writes. The directories are created lazily on the first write, so
// constructing a store -- including in tests and on a service that never
// generates -- touches nothing.
func New(outputDir, cacheDir string, packDirs map[string]string) *Store {
	return &Store{outputDir: outputDir, cacheDir: cacheDir, packDirs: maps.Clone(packDirs)}
}

// Existing reports whether slug already has a finished icon, looking in the
// directory that owns slug's namespace first and then falling back to the
// flat SRD file of its last segment. A namespace no pack owns is a
// validation error, not a miss -- that answer must land before any paid
// request, not after one. The revision is the SHA256 of the resolved file --
// the same digest Save and Read report, so a client can cache-bust on it and
// the seed importer can compare on it.
func (s *Store) Existing(ctx context.Context, slug string) (string, bool, error) {
	data, _, found, err := s.load(ctx, slug)
	if err != nil {
		var invalid *types.ValidationError
		if errors.As(err, &invalid) {
			return "", false, invalid
		}
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return revision(data), true, nil
}

// Read returns the icon for slug as it will be seeded: the slug in the image
// is the resolved key -- canonical for a namespaced slug that fell back to
// legacy SRD art -- so database keys always equal pack keys.
func (s *Store) Read(ctx context.Context, slug string) (imageasset.Image, error) {
	data, resolved, found, err := s.load(ctx, slug)
	if err != nil {
		return imageasset.Image{}, err
	}
	if !found {
		return imageasset.Image{}, types.NewNotFoundError("no icon for %q", slug)
	}
	return imageasset.Image{
		Slug:        resolved,
		Revision:    revision(data),
		ContentType: contentType,
		Data:        data,
	}, nil
}

// List returns every key the packs hold, globally qualified and sorted: bare
// keys for the flat SRD files in the base directory, "pack/local" keys for
// each mapped pack root. Only the top level of a directory is listed -- pack
// icons live flat under their root, and anything deeper is debris, not seed.
// Entries whose key could never be a slug are skipped rather than served,
// keeping temp files out of the seed set. A missing base directory is a
// missing seed pack -- an error, not an empty list, so a startup import
// fails loudly instead of seeding nothing. A missing pack root is the empty
// pack it stands for: the first generation into it creates it.
func (s *Store) List(ctx context.Context) ([]string, error) {
	keys, err := s.listDir(ctx, s.outputDir, "")
	if err != nil {
		return nil, err
	}
	for _, pack := range slices.Sorted(maps.Keys(s.packDirs)) {
		pk, err := s.listDir(ctx, s.packDirs[pack], pack)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		keys = append(keys, pk...)
	}
	slices.Sort(keys)
	return keys, nil
}

// listDir reads one pack directory flat, returning its .webp entries as
// qualified keys: prefix+"/"+name, or the bare name for the SRD base.
func (s *Store) listDir(ctx context.Context, dir, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, d := range entries {
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(name, ".webp") {
			continue
		}
		local := strings.TrimSuffix(name, ".webp")
		if !slugSegment.MatchString(local) {
			continue
		}
		if prefix != "" {
			keys = append(keys, prefix+"/"+local)
		} else {
			keys = append(keys, local)
		}
	}
	return keys, nil
}

// Save stores one generated PNG: the master goes to the cache as
// <slug>.png, the 128px webp lands flat in the directory that owns slug's
// namespace as <local>.webp. Either way a previous icon survives a failed
// save.
func (s *Store) Save(ctx context.Context, slug string, pngData []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dir, name, err := s.saveRoot(slug)
	if err != nil {
		return "", err
	}
	webp, err := s.render(slug, pngData)
	if err != nil {
		return "", err
	}
	// The cache copy lands first so a failed publish still leaves the
	// retrievable master; a cache failure aborts before the icon changes.
	if s.cacheDir != "" {
		if err := writeFile(s.cacheDir, slug+".png", pngData); err != nil {
			return "", fmt.Errorf("cache png: %w", err)
		}
	}
	if err := writeFile(dir, name, webp); err != nil {
		return "", err
	}
	return revision(webp), nil
}

// load resolves slug to pack bytes: the icon's own file in the directory
// that owns its namespace, then the flat SRD file named by its last segment.
// The resolved key is the file's key, so a fallback read reports the slug
// the bytes actually answer to.
func (s *Store) load(ctx context.Context, slug string) (data []byte, resolved string, found bool, err error) {
	if err := ctx.Err(); err != nil {
		return nil, "", false, err
	}
	candidates, err := s.resolve(slug)
	if err != nil {
		return nil, "", false, err
	}
	for _, c := range candidates {
		data, err := s.readInDir(c.dir, c.name)
		switch {
		case err == nil:
			return data, c.key, true, nil
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return nil, "", false, err
		}
	}
	return nil, "", false, nil
}

// candidate is one file a slug can resolve to: the directory that owns it,
// its flat name inside, and the key the bytes answer to.
type candidate struct {
	dir, name, key string
}

// resolve maps slug to the files it can answer from: its own icon in the
// owning pack's directory, then the flat SRD file of its last segment. A
// namespaced slug with no configured pack is refused -- the namespace names
// a repository, and guessing at one would file paid art into the wrong tree.
func (s *Store) resolve(slug string) ([]candidate, error) {
	if !slugRE.MatchString(slug) {
		return nil, types.NewValidationError("invalid icon slug %q", slug).
			Because(spellicon.ReasonInvalidRequest)
	}
	ns, local, namespaced := strings.Cut(slug, "/")
	if !namespaced {
		return []candidate{{dir: s.outputDir, name: slug + ".webp", key: slug}}, nil
	}
	dir, ok := s.packDirs[ns]
	if !ok {
		return nil, unconfiguredPack(ns, slug)
	}
	return []candidate{
		{dir: dir, name: local + ".webp", key: slug},
		{dir: s.outputDir, name: local + ".webp", key: local},
	}, nil
}

// saveRoot picks the directory and file name a slug's webp lands in: the
// owning pack root flat by local slug, or the SRD base for a bare slug.
func (s *Store) saveRoot(slug string) (dir, name string, err error) {
	cs, err := s.resolve(slug)
	if err != nil {
		return "", "", err
	}
	return cs[0].dir, cs[0].name, nil
}

// unconfiguredPack reports a namespaced slug whose namespace no pack_dirs
// entry owns.
func unconfiguredPack(ns, slug string) error {
	return types.NewValidationError("no image pack is configured for namespace %q (slug %q)", ns, slug).
		Because(spellicon.ReasonPackUnconfigured, types.Args{"pack": ns})
}

// readInDir opens dir for the length of one bounded read. A missing
// directory reports not-exist, which the callers fold into the fallback
// chain exactly like a missing file.
func (s *Store) readInDir(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readBound(root, name)
}

// render decodes the provider PNG and produces the published icon: a center
// crop to square, downscaled to IconSize with alpha preserved, encoded
// lossless.
func (s *Store) render(slug string, pngData []byte) ([]byte, error) {
	// DecodeConfig first: the size limit rejects absurd inputs before the
	// full decode pays for their pixels.
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode png for %q: %w", slug, err)
	}
	if cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width < 1 || cfg.Height < 1 {
		return nil, fmt.Errorf("unreasonable image size %dx%d for %q", cfg.Width, cfg.Height, slug)
	}
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode png for %q: %w", slug, err)
	}
	b := src.Bounds()
	// Cover semantics, the way sharp's default resize behaves: scale the
	// centered square, so a non-square master crops instead of squashing.
	if b.Dx() != b.Dy() {
		side := min(b.Dx(), b.Dy())
		x := b.Min.X + (b.Dx()-side)/2
		y := b.Min.Y + (b.Dy()-side)/2
		b = image.Rect(x, y, x+side, y+side)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, IconSize, IconSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := nativewebp.Encode(&buf, dst, nil); err != nil {
		return nil, fmt.Errorf("encode webp for %q: %w", slug, err)
	}
	return buf.Bytes(), nil
}

// writeFile atomically writes name under dir, creating the directory and any
// slug namespace on demand. The temp file is a sibling of the destination --
// rename cannot cross filesystems -- and is removed on any failure, so the
// destination is never a partial file.
func writeFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	parent := path.Dir(name)
	if parent != "." {
		if err := root.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	tmp := parent + "/.tmp-" + rand.Text()
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			root.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := root.Rename(tmp, name); err != nil {
		return err
	}
	ok = true
	return nil
}

// revision is the SHA256 of the file's bytes -- the same digest Existing,
// Read and Save report for the same content.
func revision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readBound reads a file inside root with a size cap.
func readBound(root *os.Root, name string) ([]byte, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxImageBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", name, maxImageBytes)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxImageBytes+1))
}
