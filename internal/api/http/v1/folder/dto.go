package folder

import (
	domain "github.com/promix1722/easydnd/internal/domain/character"
)

// Folder is one row of a folder listing.
type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Default marks the folder every account has and none can delete. A
	// client uses it to leave the delete control off that one row, and to
	// know where a character with no folder named will land.
	Default bool `json:"default"`
}

// folderOf converts a folder for the wire.
func folderOf(f domain.Folder) Folder {
	return Folder{ID: f.ID.String(), Name: f.Name, Default: f.Default}
}
