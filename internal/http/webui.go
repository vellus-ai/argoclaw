package http

import (
	"io/fs"
	nethttp "net/http"
	"strings"
)

// NewWebUIHandler creates an http.Handler that serves a SPA (Single Page
// Application) from an fs.FS. It:
//   - Adds a Content-Security-Policy header (baseline OWASP headers are set globally by securityHeadersMiddleware)
//   - Serves static files directly for existing, non-directory paths
//   - Falls back to index.html for unknown paths and directories — replicating
//     nginx's "try_files $uri $uri/ /index.html" for client-side routing
//   - Prevents directory listing by treating directory paths as SPA fallback
//
// Note: static assets are intentionally served without Bearer token auth so the
// login page (index.html) is accessible to unauthenticated users. Sensitive
// operations are protected at the API layer (/v1/*, /ws, /mcp/*).
func NewWebUIHandler(fsys fs.FS) nethttp.Handler {
	fileServer := nethttp.FileServer(nethttp.FS(fsys))

	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		// CSP is SPA-specific; baseline headers (HSTS, X-Frame-Options, etc.) are set globally
		// by securityHeadersMiddleware in gateway/server.go.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:")

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
			// File not found → serve index.html (SPA fallback).
			// Clone the request to avoid mutating r.URL.Path for downstream
			// middleware (logging, tracing).
			req := r.Clone(r.Context())
			req.URL.Path = "/"
			fileServer.ServeHTTP(w, req)
			return
		}
		stat, statErr := f.Stat()
		_ = f.Close()
		if statErr != nil || stat.IsDir() {
			// Directory path → SPA fallback (prevents directory listing).
			req := r.Clone(r.Context())
			req.URL.Path = "/"
			fileServer.ServeHTTP(w, req)
			return
		}

		// File exists and is not a directory → serve it directly.
		fileServer.ServeHTTP(w, r)
	})
}
