package http

import (
	"embed"
	"io/fs"
)

//go:embed all:ui_dist
var uiDistFS embed.FS

// UIDistFS returns the embedded Web UI filesystem rooted at ui_dist/.
// Returns nil if the UI was not included in the build (empty directory or .gitkeep only).
func UIDistFS() fs.FS {
	return uiDistFSFrom(uiDistFS)
}

// uiDistFSFrom extracts the Web UI sub-filesystem from the provided fs.FS.
// Separated from UIDistFS to allow unit testing with fstest.MapFS instances.
func uiDistFSFrom(fsys fs.FS) fs.FS {
	entries, err := fs.ReadDir(fsys, "ui_dist")
	if err != nil || len(entries) == 0 || (len(entries) == 1 && entries[0].Name() == ".gitkeep") {
		return nil
	}
	sub, err := fs.Sub(fsys, "ui_dist")
	if err != nil {
		return nil
	}
	return sub
}
