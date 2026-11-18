package fakegoogle

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type captured struct{ srv *http.Server }

func (c *captured) serve(_ context.Context, srv *http.Server, _ *slog.Logger) error {
	c.srv = srv
	return nil
}

func run(environ map[string]string, args ...string) (*captured, string, error) {
	c := &captured{}
	cmd := NewCommand(func(k string) string { return environ[k] }, c.serve, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return c, out.String(), err
}

func TestCommand(t *testing.T) {
	c, _, err := run(map[string]string{"CLIENT_ID": "cid", "CLIENT_SECRET": "secret", "PREFIX": "/fake-google/"},
		"--public-url", "https://demo.test/fake-google", "--upstream", "http://127.0.0.1:8080", "--addr", ":1234")
	if err != nil {
		t.Fatal(err)
	}
	if c.srv.Addr != ":1234" {
		t.Fatalf("addr %q", c.srv.Addr)
	}
	w := httptest.NewRecorder()
	c.srv.Handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/fake-google/avatar/100001", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("prefixed fake endpoint: %d", w.Code)
	}
}

func TestCommandDefaults(t *testing.T) {
	c, _, err := run(map[string]string{"PUBLIC_URL": "http://x", "CLIENT_ID": "c", "CLIENT_SECRET": "s"})
	if err != nil || c.srv.Addr != ":9090" {
		t.Fatalf("%v %+v", err, c.srv)
	}
}

func TestCommandErrors(t *testing.T) {
	full := map[string]string{"PUBLIC_URL": "http://x", "CLIENT_ID": "c", "CLIENT_SECRET": "s"}
	if _, out, err := run(nil); err == nil || !strings.Contains(out, "are required") {
		t.Fatalf("%q %v", out, err)
	}
	if _, out, err := run(full, "--upstream", "relative"); err == nil || !strings.Contains(out, "absolute URL") {
		t.Fatalf("%q %v", out, err)
	}
	if _, _, err := run(full, "unexpected"); err == nil {
		t.Fatal("positional arguments must be rejected")
	}
}
