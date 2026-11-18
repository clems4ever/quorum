package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"quorum/internal/model"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

// memStore is an in-memory SessionStore and UserSaver.
type memStore struct {
	users    map[string]model.User
	sessions map[string]string
	expires  map[string]time.Time
	err      error
}

func newMemStore() *memStore {
	return &memStore{users: map[string]model.User{}, sessions: map[string]string{}, expires: map[string]time.Time{}}
}

func (m *memStore) UpsertUser(_ context.Context, u model.User) error {
	if m.err != nil {
		return m.err
	}
	m.users[u.ID] = u
	return nil
}

func (m *memStore) CreateSession(_ context.Context, h, uid string, exp time.Time) error {
	if m.err != nil {
		return m.err
	}
	m.sessions[h], m.expires[h] = uid, exp
	return nil
}

func (m *memStore) SessionUser(_ context.Context, h string, now time.Time) (model.User, error) {
	uid, ok := m.sessions[h]
	if !ok || !m.expires[h].After(now) {
		return model.User{}, model.ErrNotFound
	}
	return m.users[uid], nil
}

func (m *memStore) DeleteSession(_ context.Context, h string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.sessions, h)
	return nil
}

type fakeProvider struct {
	user model.User
	err  error
	code string
}

func (f *fakeProvider) AuthCodeURL(state string) string {
	return "https://idp.test/auth?state=" + state
}

func (f *fakeProvider) Exchange(_ context.Context, code string) (model.User, error) {
	f.code = code
	return f.user, f.err
}

func seq(vals ...string) func(int) (string, error) {
	return func(int) (string, error) {
		v := vals[0]
		vals = vals[1:]
		return v, nil
	}
}

func newHandler(st *memStore, p Provider, random func(int) (string, error)) (*Handler, *http.ServeMux) {
	h := &Handler{
		Provider: p,
		Sessions: &Sessions{Store: st, TTL: time.Hour, Secure: true, Now: func() time.Time { return t0 }, Random: random},
		Users:    st,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

func do(mux http.Handler, method, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLoginFlow(t *testing.T) {
	st := newMemStore()
	ada := model.User{ID: "google:1", Email: "ada@example.com", Name: "Ada"}
	p := &fakeProvider{user: ada}
	h, mux := newHandler(st, p, seq("STATE", "TOKEN"))

	w := do(mux, "GET", "/auth/login?next=/events/abc")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://idp.test/auth?state=STATE" {
		t.Fatalf("login: %d %s", w.Code, w.Header().Get("Location"))
	}
	state := cookie(w, stateCookie)
	if state == nil || state.Value != "STATE|/events/abc" || !state.HttpOnly || !state.Secure {
		t.Fatalf("state cookie: %+v", state)
	}

	w = do(mux, "GET", "/auth/callback?state=STATE&code=CODE", state)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/events/abc" {
		t.Fatalf("callback: %d %s %s", w.Code, w.Header().Get("Location"), w.Body)
	}
	if p.code != "CODE" || st.users[ada.ID] != ada {
		t.Fatalf("user not saved: %+v", st.users)
	}
	sess := cookie(w, SessionCookie)
	if sess == nil || sess.Value != "TOKEN" || !sess.Expires.Equal(t0.Add(time.Hour)) {
		t.Fatalf("session cookie: %+v", sess)
	}

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	r.AddCookie(sess)
	if u, err := h.Sessions.User(r); err != nil || u != ada {
		t.Fatalf("session user: %+v %v", u, err)
	}

	w = do(mux, "POST", "/auth/logout", sess)
	if w.Code != http.StatusNoContent || cookie(w, SessionCookie).MaxAge != -1 {
		t.Fatalf("logout: %d", w.Code)
	}
	if _, err := h.Sessions.User(r); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("session should be gone: %v", err)
	}
	// Logging out without a cookie is fine.
	if w := do(mux, "POST", "/auth/logout"); w.Code != http.StatusNoContent {
		t.Fatalf("logout without cookie: %d", w.Code)
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":              "/",
		"/x?y=1":        "/x?y=1",
		"//evil.com":    "/",
		"/\\evil.com":   "/",
		"https://e.com": "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCallbackErrors(t *testing.T) {
	state := &http.Cookie{Name: stateCookie, Value: "S|/"}
	cases := []struct {
		name   string
		target string
		cookie *http.Cookie
		p      *fakeProvider
		stErr  error
		random []string
		want   int
		loc    string
	}{
		{name: "no cookie", target: "/auth/callback?state=S", want: 400},
		{name: "state mismatch", target: "/auth/callback?state=X", cookie: state, want: 400},
		{name: "empty state", target: "/auth/callback?state=", cookie: &http.Cookie{Name: stateCookie, Value: "|/"}, want: 400},
		{name: "denied", target: "/auth/callback?state=S&error=access_denied", cookie: &http.Cookie{Name: stateCookie, Value: "S|/e/1"}, want: 302, loc: "/e/1"},
		{name: "exchange fails", target: "/auth/callback?state=S&code=C", cookie: state, p: &fakeProvider{err: errors.New("boom")}, want: 401},
		{name: "save fails", target: "/auth/callback?state=S&code=C", cookie: state, stErr: errors.New("db"), want: 500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := newMemStore()
			st.err = c.stErr
			p := c.p
			if p == nil {
				p = &fakeProvider{user: model.User{ID: "u"}}
			}
			_, mux := newHandler(st, p, seq("T"))
			var cookies []*http.Cookie
			if c.cookie != nil {
				cookies = append(cookies, c.cookie)
			}
			w := do(mux, "GET", c.target, cookies...)
			if w.Code != c.want {
				t.Fatalf("status %d, want %d: %s", w.Code, c.want, w.Body)
			}
			if c.loc != "" && w.Header().Get("Location") != c.loc {
				t.Fatalf("location %q", w.Header().Get("Location"))
			}
		})
	}
}

func TestSessionStartFailures(t *testing.T) {
	st := newMemStore()
	failing := func(int) (string, error) { return "", errors.New("no entropy") }
	_, mux := newHandler(st, &fakeProvider{}, failing)
	if w := do(mux, "GET", "/auth/login"); w.Code != 500 {
		t.Fatalf("login with failing random: %d", w.Code)
	}

	state := &http.Cookie{Name: stateCookie, Value: "S|/"}
	_, mux = newHandler(st, &fakeProvider{user: model.User{ID: "u"}}, failing)
	if w := do(mux, "GET", "/auth/callback?state=S&code=C", state); w.Code != 500 {
		t.Fatalf("callback with failing random: %d", w.Code)
	}

	// Store fails while creating the session (but not while saving the user).
	h, mux := newHandler(st, &fakeProvider{user: model.User{ID: "u"}}, seq("T"))
	h.Sessions.Store = &failingSessions{memStore: st}
	if w := do(mux, "GET", "/auth/callback?state=S&code=C", state); w.Code != 500 {
		t.Fatalf("callback with failing session store: %d", w.Code)
	}
	if w := do(mux, "POST", "/auth/logout", &http.Cookie{Name: SessionCookie, Value: "x"}); w.Code != 500 {
		t.Fatalf("logout with failing store: %d", w.Code)
	}
}

type failingSessions struct{ *memStore }

func (f *failingSessions) CreateSession(context.Context, string, string, time.Time) error {
	return errors.New("db")
}

func (f *failingSessions) DeleteSession(context.Context, string) error { return errors.New("db") }

func TestSessionUserWithoutCookie(t *testing.T) {
	s := &Sessions{Store: newMemStore(), Now: time.Now}
	if _, err := s.User(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestRandomToken(t *testing.T) {
	a, err := RandomToken(16)
	if err != nil || len(a) != 22 {
		t.Fatalf("%q %v", a, err)
	}
	b, _ := RandomToken(16)
	if a == b {
		t.Fatal("tokens should differ")
	}
}

func write(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Error(err)
	}
}

// idp is a minimal OIDC server used to test the Google provider.
func idp(t *testing.T, userinfo func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := r.ParseForm(); err != nil || r.PostForm.Get("code") != "good" || r.PostForm.Get("client_secret") != "secret" {
			w.WriteHeader(http.StatusBadRequest)
			write(t, w, `{"error":"invalid_grant"}`)
			return
		}
		write(t, w, `{"access_token":"AT","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userinfo(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func google(srv *httptest.Server) *Google {
	return NewGoogle(GoogleConfig{
		ClientID: "cid", ClientSecret: "secret", RedirectURL: "https://app.test/auth/callback",
		AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token", UserInfoURL: srv.URL + "/userinfo",
		HTTPClient: srv.Client(),
	})
}

func TestGoogleDefaults(t *testing.T) {
	g := NewGoogle(GoogleConfig{ClientID: "cid", RedirectURL: "https://app.test/cb"})
	u, err := url.Parse(g.AuthCodeURL("st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != GoogleAuthURL || q.Get("state") != "st" || q.Get("client_id") != "cid" ||
		q.Get("scope") != "openid email profile" || q.Get("prompt") != "select_account" || q.Get("redirect_uri") != "https://app.test/cb" {
		t.Fatalf("auth url: %s", u)
	}
	if g.userInfoURL != GoogleUserInfoURL || g.client != http.DefaultClient {
		t.Fatal("defaults not applied")
	}
}

func TestGoogleExchange(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		userinfo string
		status   int
		want     model.User
		err      string
	}{
		{name: "ok", code: "good", userinfo: `{"sub":"42","email":"a@x.com","email_verified":true,"name":"Ada","picture":"pic"}`,
			want: model.User{ID: "google:42", Email: "a@x.com", Name: "Ada", Picture: "pic"}},
		{name: "name defaults to email", code: "good", userinfo: `{"sub":"42","email":"a@x.com","email_verified":true}`,
			want: model.User{ID: "google:42", Email: "a@x.com", Name: "a@x.com"}},
		{name: "bad code", code: "bad", err: "exchange code"},
		{name: "userinfo status", code: "good", status: 500, userinfo: "oops", err: "status 500"},
		{name: "userinfo json", code: "good", userinfo: `{`, err: "decode userinfo"},
		{name: "missing sub", code: "good", userinfo: `{"email":"a@x.com","email_verified":true}`, err: "missing subject"},
		{name: "unverified", code: "good", userinfo: `{"sub":"1","email":"a@x.com"}`, err: "not verified"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := idp(t, func(w http.ResponseWriter, _ *http.Request) {
				if c.status != 0 {
					w.WriteHeader(c.status)
				}
				write(t, w, c.userinfo)
			})
			u, err := google(srv).Exchange(context.Background(), c.code)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err = %v, want %q", err, c.err)
				}
				return
			}
			if err != nil || u != c.want {
				t.Fatalf("got %+v %v", u, err)
			}
		})
	}
}

func TestGoogleUserinfoUnreachable(t *testing.T) {
	srv := idp(t, nil)
	g := google(srv)
	g.userInfoURL = "http://127.0.0.1:1/userinfo"
	if _, err := g.Exchange(context.Background(), "good"); err == nil || !strings.Contains(err.Error(), "fetch userinfo") {
		t.Fatalf("got %v", err)
	}
}
