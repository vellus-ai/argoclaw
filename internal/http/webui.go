package http

import (
	"io/fs"
	nethttp "net/http"
	"strings"
)

// NewWebUIHandler creates an http.Handler that serves a SPA (Single Page
// Application) from an fs.FS. It serves static files directly and falls back
// to index.html for any path that doesn't match a real file — replicating
// nginx's "try_files $uri $uri/ /index.html" for client-side routing.
func NewWebUIHandler(fsys fs.FS) nethttp.Handler {
	fileServer := nethttp.FileServer(nethttp.FS(fsys))

	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		// Clean path
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = strings.TrimPrefix(path, "/")
		}

		// Try to open the file
		f, err := fsys.Open(path)
		if err != nil {
			// File not found → serve index.html (SPA fallback)
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		// File exists → serve it directly
		fileServer.ServeHTTP(w, r)
	})
}
