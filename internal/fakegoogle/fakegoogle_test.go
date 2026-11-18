package fakegoogle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func decode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Error(err)
	}
}

func newServer() (http.Handler, *time.Time) {
	now := t0
	n := 0
	s := New(Config{
		PublicURL: "https://fake.test/", ClientID: "cid", ClientSecret: "secret",
		Now: func() time.Time { return now },
		Random: func() string {
			n++
			return fmt.Sprintf("tok%d", n)
		},
	})
	return s.Handler(), &now
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func form(target string, v url.Values) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func base() url.Values {
	return url.Values{"client_id": {"cid"}, "redirect_uri": {"https://app.test/cb?x=1"}, "state": {"st"}}
}

func TestDefaults(t *testing.T) {
	s := New(Config{})
	if len(s.cfg.Profiles) != len(DefaultProfiles) || s.cfg.Now == nil || len(s.cfg.Random()) != 32 {
		t.Fatal("defaults not applied")
	}
}

func TestInitials(t *testing.T) {
	for in, want := range map[string]string{"ada lovelace": "AL", "Grace Brewster Hopper": "GB", "Linus": "L", "": ""} {
		if got := (Profile{Name: in}).Initials(); got != want {
			t.Errorf("Initials(%q) = %q", in, got)
		}
	}
}

func TestAuthorizePage(t *testing.T) {
	h, _ := newServer()
	q := base()
	q.Set("response_type", "code")
	w := serve(h, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/o/oauth2/v2/auth?"+q.Encode(), nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{"Choose an account", "Ada Lovelace", "https://fake.test/avatar/100001", `value="st"`, "Use another account"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}

	bad := []url.Values{
		{"client_id": {"other"}, "redirect_uri": {"https://app.test/cb"}, "response_type": {"code"}},
		{"client_id": {"cid"}, "redirect_uri": {"/relative"}, "response_type": {"code"}},
		{"client_id": {"cid"}, "redirect_uri": {"https://app.test/cb"}, "response_type": {"token"}},
	}
	for _, q := range bad {
		if w := serve(h, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/o/oauth2/v2/auth?"+q.Encode(), nil)); w.Code != 400 {
			t.Errorf("%v: status %d", q, w.Code)
		}
	}
}

func TestAuthorizePageInFrench(t *testing.T) {
	h, _ := newServer()
	q := base()
	q.Set("response_type", "code")
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/o/oauth2/v2/auth?"+q.Encode(), nil)
	r.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en;q=0.8")
	body := serve(h, r).Body.String()
	for _, want := range []string{`lang="fr"`, "Choisissez un compte", "Utiliser un autre compte", "Annuler"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestLanguage(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "en",
		"fr":                     "fr",
		"de-DE,fr-CA;q=0.8":      "fr",
		"en-GB,fr;q=0.5":         "en",
		"es, it;q=0.5":           "en",
		" FR-be ; q=1, en;q=0.1": "fr",
	} {
		if got := language(in); got != want {
			t.Errorf("language(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFullFlow(t *testing.T) {
	h, _ := newServer()
	v := base()
	v.Set("sub", "100001")
	w := serve(h, form("/o/oauth2/v2/auth", v))
	if w.Code != http.StatusFound {
		t.Fatalf("approve: %d %s", w.Code, w.Body)
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	if loc.Host != "app.test" || loc.Query().Get("code") != "tok1" || loc.Query().Get("state") != "st" || loc.Query().Get("x") != "1" {
		t.Fatalf("redirect: %s", loc)
	}

	tv := url.Values{"grant_type": {"authorization_code"}, "code": {"tok1"}, "redirect_uri": {"https://app.test/cb?x=1"},
		"client_id": {"cid"}, "client_secret": {"secret"}}
	w = serve(h, form("/token", tv))
	if w.Code != 200 {
		t.Fatalf("token: %d %s", w.Code, w.Body)
	}
	var tok map[string]any
	decode(t, w.Body.Bytes(), &tok)
	if tok["access_token"] != "tok2" || tok["token_type"] != "Bearer" {
		t.Fatalf("token: %v", tok)
	}

	// Codes are single use.
	if w := serve(h, form("/token", tv)); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_grant") {
		t.Fatalf("reused code: %d %s", w.Code, w.Body)
	}

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/userinfo", nil)
	r.Header.Set("Authorization", "Bearer tok2")
	w = serve(h, r)
	var info map[string]any
	decode(t, w.Body.Bytes(), &info)
	if info["sub"] != "100001" || info["email"] != "ada@example.com" || info["email_verified"] != true ||
		info["picture"] != "https://fake.test/avatar/100001" {
		t.Fatalf("userinfo: %v", info)
	}

	r = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/userinfo", nil)
	r.Header.Set("Authorization", "Bearer nope")
	if w := serve(h, r); w.Code != 401 {
		t.Fatalf("bad token: %d", w.Code)
	}
}

func TestCustomAccountAndAvatar(t *testing.T) {
	h, _ := newServer()
	v := base()
	v.Set("name", " Jane Doe ")
	v.Set("email", "Jane@Example.com")
	w := serve(h, form("/o/oauth2/v2/auth", v))
	if w.Code != http.StatusFound {
		t.Fatalf("approve custom: %d %s", w.Code, w.Body)
	}
	tv := url.Values{"grant_type": {"authorization_code"}, "code": {"tok1"}, "redirect_uri": {"https://app.test/cb?x=1"}}
	r := form("/token", tv)
	r.SetBasicAuth("cid", "secret")
	w = serve(h, r)
	if w.Code != 200 {
		t.Fatalf("token with basic auth: %d %s", w.Code, w.Body)
	}
	r = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/userinfo", nil)
	r.Header.Set("Authorization", "Bearer tok2")
	var info struct {
		Sub, Email, Name, Picture string
	}
	decode(t, serve(h, r).Body.Bytes(), &info)
	if info.Email != "jane@example.com" || info.Name != "Jane Doe" || !strings.HasPrefix(info.Sub, "custom-") {
		t.Fatalf("userinfo: %+v", info)
	}

	avatar := strings.TrimPrefix(info.Picture, "https://fake.test")
	w = serve(h, httptest.NewRequestWithContext(context.Background(), http.MethodGet, avatar, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(w.Body.String(), ">JD<") {
		t.Fatalf("custom avatar: %d %s", w.Code, w.Body)
	}
	w = serve(h, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/avatar/100002", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), ">AT<") {
		t.Fatalf("avatar: %d %s", w.Code, w.Body)
	}
	if w := serve(h, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/avatar/nobody", nil)); w.Code != 404 {
		t.Fatalf("unknown avatar: %d", w.Code)
	}
}

func TestApproveErrors(t *testing.T) {
	h, _ := newServer()
	cases := map[string]url.Values{
		"bad client":    {"client_id": {"x"}, "redirect_uri": {"https://a.test/"}},
		"unknown sub":   {"client_id": {"cid"}, "redirect_uri": {"https://a.test/"}, "sub": {"999"}},
		"invalid email": {"client_id": {"cid"}, "redirect_uri": {"https://a.test/"}, "name": {"J"}, "email": {"nope"}},
		"missing name":  {"client_id": {"cid"}, "redirect_uri": {"https://a.test/"}, "email": {"j@x.com"}},
		"name too long": {"client_id": {"cid"}, "redirect_uri": {"https://a.test/"}, "name": {strings.Repeat("a", 81)}, "email": {"j@x.com"}},
	}
	for name, v := range cases {
		if w := serve(h, form("/o/oauth2/v2/auth", v)); w.Code != 400 {
			t.Errorf("%s: status %d", name, w.Code)
		}
	}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/o/oauth2/v2/auth?%zz", strings.NewReader("a=%zz"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := serve(h, r); w.Code != 400 {
		t.Errorf("malformed form: %d", w.Code)
	}

	v := base()
	v.Set("action", "cancel")
	w := serve(h, form("/o/oauth2/v2/auth", v))
	loc, _ := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || loc.Query().Get("error") != "access_denied" || loc.Query().Get("state") != "st" {
		t.Fatalf("cancel: %d %s", w.Code, loc)
	}
}

func TestTokenErrors(t *testing.T) {
	h, now := newServer()
	v := base()
	v.Set("sub", "100001")
	serve(h, form("/o/oauth2/v2/auth", v)) // code tok1
	good := func() url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "code": {"tok1"}, "redirect_uri": {"https://app.test/cb?x=1"},
			"client_id": {"cid"}, "client_secret": {"secret"}}
	}
	check := func(name string, v url.Values, want string) {
		t.Helper()
		w := serve(h, form("/token", v))
		if w.Code != 400 || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	bad := good()
	bad.Set("client_secret", "nope")
	check("secret", bad, "invalid_client")
	bad = good()
	bad.Set("grant_type", "password")
	check("grant type", bad, "unsupported_grant_type")
	bad = good()
	bad.Set("redirect_uri", "https://evil.test/")
	check("redirect mismatch", bad, "invalid_grant") // consumes tok1

	serve(h, form("/o/oauth2/v2/auth", v)) // code tok2
	*now = now.Add(time.Hour)
	expired := good()
	expired.Set("code", "tok2")
	check("expired", expired, "invalid_grant")

	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/token", strings.NewReader("a=%zz"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := serve(h, r); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_request") {
		t.Errorf("malformed: %d %s", w.Code, w.Body)
	}
}

func TestGateway(t *testing.T) {
	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { write(t, w, "fake:"+r.URL.Path) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		write(t, w, "app:"+r.URL.Path+":"+r.Header.Get("X-Forwarded-Host"))
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)

	get := func(h http.Handler, path string) (int, string) {
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		r.Host = "demo.test"
		w := serve(h, r)
		return w.Code, w.Body.String()
	}
	h := Gateway("/fake-google", fake, u)
	if _, body := get(h, "/fake-google/token"); body != "fake:/token" {
		t.Errorf("prefixed: %q", body)
	}
	if _, body := get(h, "/events/1"); body != "app:/events/1:demo.test" {
		t.Errorf("proxied: %q", body)
	}
	if code, _ := get(Gateway("/fake-google", fake, nil), "/events/1"); code != 404 {
		t.Errorf("no upstream: %d", code)
	}
	if _, body := get(Gateway("", fake, nil), "/token"); body != "fake:/token" {
		t.Errorf("root: %q", body)
	}
}
