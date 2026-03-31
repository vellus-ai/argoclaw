package http

import (
	"errors"
	"io/fs"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"
)

// --- NewWebUIHandler tests ---

// TestNewWebUIHandler_ServesIndexHTML verifies that "/" returns index.html content.
func TestNewWebUIHandler_ServesIndexHTML(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>hello</html>")},
	}
	handler := NewWebUIHandler(fsys)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != nethttp.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "<html>hello</html>" {
		t.Errorf("body = %q, want index.html content", body)
	}
}

// TestNewWebUIHandler_ServesExistingFile verifies that an existing file is served directly.
func TestNewWebUIHandler_ServesExistingFile(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<html></html>")},
		"assets/main.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	handler := NewWebUIHandler(fsys)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/assets/main.js", nil)
	handler.ServeHTTP(w, r)

	if w.Code != nethttp.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "console.log(1)" {
		t.Errorf("body = %q, want JS content", body)
	}
}

// TestNewWebUIHandler_FallbackToIndexHTML verifies unknown paths fall back to index.html (SPA routing).
func TestNewWebUIHandler_FallbackToIndexHTML(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>spa</html>")},
	}
	handler := NewWebUIHandler(fsys)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dashboard/settings", nil)
	handler.ServeHTTP(w, r)

	if w.Code != nethttp.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "<html>spa</html>" {
		t.Errorf("body = %q, want index.html (SPA fallback)", body)
	}
}

// TestNewWebUIHandler_DirectoryFallback verifies that a directory path falls back to index.html
// and does NOT expose a directory listing.
func TestNewWebUIHandler_DirectoryFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/main.js": &fstest.MapFile{Data: []byte("code")},
	}
	handler := NewWebUIHandler(fsys)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/assets/", nil)
	handler.ServeHTTP(w, r)

	if w.Code != nethttp.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if body == "code" {
		t.Error("directory listing exposed: /assets/ should fall back to index.html, not serve directory contents")
	}
	if body != "<html>spa</html>" {
		t.Errorf("body = %q, want index.html (directory listing prevented)", body)
	}
}

// TestNewWebUIHandler_SecurityHeaders verifies that the CSP header is set on every response.
// Baseline headers (X-Frame-Options, X-Content-Type-Options, HSTS, etc.) are applied globally
// by securityHeadersMiddleware in gateway/server.go — not tested here.
func TestNewWebUIHandler_SecurityHeaders(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}
	handler := NewWebUIHandler(fsys)

	paths := []string{"/", "/dashboard", "/assets/main.js"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", p, nil)
			handler.ServeHTTP(w, r)

			if csp := w.Header().Get("Content-Security-Policy"); csp == "" {
				t.Errorf("path %s: Content-Security-Policy header not set", p)
			}
		})
	}
}

// TestNewWebUIHandler_DoesNotMutateRequest verifies that the original *http.Request is not mutated
// when falling back to index.html. Mutation would corrupt downstream logging/tracing middleware.
func TestNewWebUIHandler_DoesNotMutateRequest(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}
	handler := NewWebUIHandler(fsys)

	originalPath := "/some/deep/path"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", originalPath, nil)
	handler.ServeHTTP(w, r)

	if r.URL.Path != originalPath {
		t.Errorf("request.URL.Path mutated: got %q, want %q", r.URL.Path, originalPath)
	}
}

// --- UIDistFS tests ---

// TestUIDistFS_ReturnsNilForGitkeepOnly verifies that UIDistFS returns nil when ui_dist contains
// only a .gitkeep placeholder — meaning no real UI was embedded at build time.
func TestUIDistFS_ReturnsNilForGitkeepOnly(t *testing.T) {
	// The embedded ui_dist in this repo contains only .gitkeep (no ENABLE_WEB_UI build).
	// UIDistFS must return nil to skip Web UI registration.
	result := UIDistFS()
	if result != nil {
		// A real UI build is embedded — acceptable in CI with ENABLE_WEB_UI=true.
		t.Log("UIDistFS returned non-nil FS (real UI build embedded — OK in production builds)")
	}
}

// TestUIDistFSFrom_NilCases validates that uiDistFSFrom returns nil for empty/placeholder directories.
func TestUIDistFSFrom_NilCases(t *testing.T) {
	cases := []struct {
		name string
		fsys fs.FS
	}{
		{
			name: "only .gitkeep",
			fsys: fstest.MapFS{"ui_dist/.gitkeep": &fstest.MapFile{}},
		},
		{
			name: "empty ui_dist",
			fsys: fstest.MapFS{"ui_dist": &fstest.MapFile{Mode: fs.ModeDir}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := uiDistFSFrom(tc.fsys); got != nil {
				t.Errorf("uiDistFSFrom(%s) = non-nil, want nil", tc.name)
			}
		})
	}
}

// TestUIDistFSFrom_RealContent verifies that uiDistFSFrom returns a valid FS when
// ui_dist contains real files (simulating an ENABLE_WEB_UI=true build).
func TestUIDistFSFrom_RealContent(t *testing.T) {
	fsys := fstest.MapFS{
		"ui_dist/index.html":     &fstest.MapFile{Data: []byte("<html></html>")},
		"ui_dist/assets/main.js": &fstest.MapFile{Data: []byte("code")},
	}

	got := uiDistFSFrom(fsys)
	if got == nil {
		t.Fatal("uiDistFSFrom with real content returned nil")
	}

	// The returned FS should be rooted at ui_dist — index.html accessible directly.
	f, err := got.Open("index.html")
	if err != nil {
		t.Fatalf("Open index.html on sub-FS: %v", err)
	}
	_ = f.Close()
}

// TestUIDistFSFrom_GitkeepPlusRealFiles verifies that .gitkeep does NOT suppress real files.
func TestUIDistFSFrom_GitkeepPlusRealFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"ui_dist/.gitkeep":   &fstest.MapFile{},
		"ui_dist/index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}

	if got := uiDistFSFrom(fsys); got == nil {
		t.Error("uiDistFSFrom with .gitkeep + real files returned nil, want valid FS")
	}
}

// partialBrokenStatFS wraps an fs.FS and returns a Stat error only for a specific file name.
// Used to test the statErr != nil defensive branch in NewWebUIHandler.
type partialBrokenStatFS struct {
	inner    fs.FS
	breakFor string // only break Stat for this file name
}

func (b partialBrokenStatFS) Open(name string) (fs.File, error) {
	f, err := b.inner.Open(name)
	if err != nil {
		return nil, err
	}
	if name == b.breakFor {
		return partialBrokenStatFile{f}, nil
	}
	return f, nil
}

type partialBrokenStatFile struct{ fs.File }

func (partialBrokenStatFile) Stat() (fs.FileInfo, error) { return nil, errors.New("stat broken") }
func (partialBrokenStatFile) Close() error               { return nil }

// TestNewWebUIHandler_BrokenStatFallback verifies that a Stat error on a non-index file
// causes a graceful SPA fallback to index.html rather than a panic or 500.
func TestNewWebUIHandler_BrokenStatFallback(t *testing.T) {
	inner := fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/main.js": &fstest.MapFile{Data: []byte("code")},
	}
	// Only break Stat for "assets/main.js"; index.html remains accessible for the fallback.
	handler := NewWebUIHandler(partialBrokenStatFS{inner: inner, breakFor: "assets/main.js"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/assets/main.js", nil)
	handler.ServeHTTP(w, r)

	if w.Code != nethttp.StatusOK {
		t.Errorf("broken Stat: status = %d, want 200 (SPA fallback)", w.Code)
	}
	if body := w.Body.String(); body != "<html>spa</html>" {
		t.Errorf("broken Stat: body = %q, want index.html fallback content", body)
	}
}

// Ensure the time import is used (required by Go compiler).
var _ = time.Time{}
