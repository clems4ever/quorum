package static

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":     {Data: []byte("<html>app</html>")},
		"favicon.svg":    {Data: []byte("<svg/>")},
		"assets/app.js":  {Data: []byte("console.log(1)")},
		"assets/sub/x.c": {Data: []byte("x")},
	}
	h := Handler(fsys)
	cases := []struct {
		path, body, cache string
		status            int
	}{
		{"/", "<html>app</html>", "no-cache", 200},
		{"/events/abc", "<html>app</html>", "no-cache", 200},
		{"/assets", "<html>app</html>", "no-cache", 200},
		{"/favicon.svg", "<svg/>", "no-cache", 200},
		{"/assets/app.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/assets/missing.js", "", "", 404},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, c.path, nil))
		if w.Code != c.status {
			t.Errorf("%s: status %d", c.path, w.Code)
			continue
		}
		if c.status != http.StatusOK {
			continue
		}
		if w.Body.String() != c.body || w.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: body %q cache %q", c.path, w.Body, w.Header().Get("Cache-Control"))
		}
	}
}
