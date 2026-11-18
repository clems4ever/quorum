// Package static serves the single page application.
package static

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler serves files from fsys and falls back to index.html for client
// side routes. Hashed assets are cached aggressively.
func Handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		st, err := fs.Stat(fsys, name)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && st.IsDir()) {
			if path.Ext(name) != "" && !strings.HasSuffix(name, ".html") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
