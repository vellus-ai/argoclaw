package http

import (
	"embed"
	"io/fs"
)

//go:embed all:ui_dist
var uiDistFS embed.FS

// UIDistFS returns the embedded Web UI filesystem rooted at ui_dist/.
// Returns nil if the UI was not included in the build (empty directory).
func UIDistFS() fs.FS {
	entries, err := fs.ReadDir(uiDistFS, "ui_dist")
	if err != nil || len(entries) == 0 || (len(entries) == 1 && entries[0].Name() == ".gitkeep") {
		return nil
	}
	sub, err := fs.Sub(uiDistFS, "ui_dist")
	if err != nil {
		return nil
	}
	return sub
}
