package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quorum/internal/fakegoogle"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaultDeps(t *testing.T) {
	d := DefaultDeps()
	id, err := d.NewID()
	if err != nil || len(id) != 12 || d.Now().IsZero() || d.Logger == nil || d.HTTPClient == nil {
		t.Fatalf("%q %v", id, err)
	}
}

func quietDeps() Deps {
	d := DefaultDeps()
	d.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return d
}

// result is the part of an HTTP response the tests look at.
type result struct {
	status int
	header http.Header
	url    *url.URL // final URL, after redirects
	body   string
}

// fetch performs a request and returns its (closed) response.
func fetch(t *testing.T, client *http.Client, method, target, contentType, body string) result {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{status: resp.StatusCode, header: resp.Header, url: resp.Request.URL, body: string(data)}
}

// TestEndToEnd signs in through the fake Google server and creates an event.
func TestEndToEnd(t *testing.T) {
	static := t.TempDir()
	if err := os.WriteFile(filepath.Join(static, "index.html"), []byte("<html>spa</html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	idp := httptest.NewUnstartedServer(nil)
	idp.Start()
	defer idp.Close()
	fake := fakegoogle.New(fakegoogle.Config{PublicURL: idp.URL, ClientID: "cid", ClientSecret: "secret"})
	idp.Config.Handler = fake.Handler()

	appSrv := httptest.NewUnstartedServer(nil)
	appSrv.Start()
	defer appSrv.Close()
	a, err := New(context.Background(), Config{
		PublicURL: appSrv.URL, DatabasePath: ":memory:", StaticDir: static,
		GoogleClientID: "cid", GoogleClientSecret: "secret",
		GoogleAuthURL: idp.URL + "/o/oauth2/v2/auth", GoogleTokenURL: idp.URL + "/token", GoogleUserInfoURL: idp.URL + "/v1/userinfo",
		SessionTTL: time.Hour,
	}, quietDeps())
	if err != nil {
		t.Fatal(err)
	}
	defer mustClose(t, a)
	appSrv.Config.Handler = a.Handler

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	// The SPA and health check are served.
	res := fetch(t, client, http.MethodGet, appSrv.URL+"/events/xyz", "", "")
	if res.body != "<html>spa</html>" || res.header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("spa: %s", res.body)
	}
	if res := fetch(t, client, http.MethodGet, appSrv.URL+"/healthz", "", ""); res.status != http.StatusOK {
		t.Fatalf("healthz: %d", res.status)
	}

	// Start sign-in: we land on the fake account chooser.
	res = fetch(t, client, http.MethodGet, appSrv.URL+"/auth/login?next=/dashboard", "", "")
	if !strings.Contains(res.body, "Grace Hopper") {
		t.Fatalf("chooser page: %s", res.body)
	}
	q := res.url.Query()
	// "Click" Grace's profile.
	form := url.Values{
		"client_id": {q.Get("client_id")}, "redirect_uri": {q.Get("redirect_uri")}, "state": {q.Get("state")}, "sub": {"100003"},
	}
	res = fetch(t, client, http.MethodPost, res.url.String(), "application/x-www-form-urlencoded", form.Encode())
	if res.url.Path != "/dashboard" {
		t.Fatalf("landed on %s", res.url)
	}

	res = fetch(t, client, http.MethodGet, appSrv.URL+"/api/me", "", "")
	var me map[string]string
	if err := json.Unmarshal([]byte(res.body), &me); err != nil {
		t.Fatal(err)
	}
	if me["name"] != "Grace Hopper" || me["id"] != "google:100003" {
		t.Fatalf("me: %v", me)
	}

	res = fetch(t, client, http.MethodPost, appSrv.URL+"/api/events", "application/json",
		`{"title":"Offsite","mode":"days","slots":[{"start":"2026-12-01T00:00:00Z","end":"2026-12-02T00:00:00Z"}]}`)
	if res.status != http.StatusCreated {
		t.Fatalf("create: %d %s", res.status, res.body)
	}
}

func mustClose(t *testing.T, a *App) {
	t.Helper()
	if err := a.Close(); err != nil {
		t.Error(err)
	}
}

func TestHealthzUnhealthy(t *testing.T) {
	a, err := New(context.Background(), Config{DatabasePath: ":memory:", StaticDir: t.TempDir(), PublicURL: "https://x"}, quietDeps())
	if err != nil {
		t.Fatal(err)
	}
	mustClose(t, a)
	w := httptest.NewRecorder()
	a.Handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", w.Code)
	}
}

func TestNewFailsOnBadDatabase(t *testing.T) {
	if _, err := New(context.Background(), Config{DatabasePath: filepath.Join(t.TempDir(), "no", "db")}, quietDeps()); err == nil {
		t.Fatal("expected error")
	}
	if err := Run(context.Background(), Config{DatabasePath: filepath.Join(t.TempDir(), "no", "db")}, quietDeps()); err == nil {
		t.Fatal("expected error")
	}
}

func freeAddr(t *testing.T) string {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitHealthy(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/healthz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunAndShutdown(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		done <- Run(ctx, Config{Addr: addr, DatabasePath: ":memory:", StaticDir: t.TempDir()}, quietDeps())
	}()
	waitHealthy(t, addr)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestLogRequests(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := logRequests(logger, time.Now, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	for _, p := range []string{"/api/missing", "/auth/login", "/assets/app.js"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, nil))
	}
	out := buf.String()
	if !strings.Contains(out, "path=/api/missing status=404") || !strings.Contains(out, "path=/auth/login status=200") {
		t.Fatalf("unexpected log: %s", out)
	}
	if strings.Contains(out, "/assets/") {
		t.Fatalf("static assets should not be logged: %s", out)
	}
}
