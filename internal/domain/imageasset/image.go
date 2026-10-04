// Package imageasset defines persisted artwork as opaque content-addressed
// bytes. Who drew the image, where the file copy lives and what request
// served it are adapter concerns; the domain only names the bytes.
package imageasset

import "slices"

// Image is one stored picture: the slug it is addressed by, the revision that
// tells two versions of it apart, its MIME type and its bytes.
//
// Revision is content-derived -- the file adapter uses the SHA256 of Data --
// so a re-import of unchanged bytes is detectable, and skippable, without
// reading anything back.
type Image struct {
	Slug, Revision, ContentType string
	Data                        []byte
}

// Clone returns a copy whose Data shares no memory with the original, so a
// caller mutating the bytes it was handed cannot rewrite what the repository
// is holding.
func (i Image) Clone() Image {
	i.Data = slices.Clone(i.Data)
	return i
}
