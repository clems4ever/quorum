// Package fakegoogle implements a tiny stand-in for Google's OAuth2/OIDC
// endpoints. It lets demos sign in by clicking a profile instead of using a
// real Google account. It is only used by the cmd/fakegoogle binary.
package fakegoogle

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html
var templates embed.FS

var pages = template.Must(template.ParseFS(templates, "templates/*.html"))

// Profile is a fake Google account.
type Profile struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Color string `json:"-"`
}

// Initials returns up to two initials of the profile name.
func (p Profile) Initials() string {
	var out []rune
	for _, f := range strings.Fields(p.Name) {
		out = append(out, []rune(f)[0])
		if len(out) == 2 {
			break
		}
	}
	return strings.ToUpper(string(out))
}

// DefaultProfiles are the accounts offered on the sign-in page.
var DefaultProfiles = []Profile{
	{Sub: "100001", Name: "Ada Lovelace", Email: "ada@example.com", Color: "#7048e8"},
	{Sub: "100002", Name: "Alan Turing", Email: "alan@example.com", Color: "#1c7ed6"},
	{Sub: "100003", Name: "Grace Hopper", Email: "grace@example.com", Color: "#0ca678"},
	{Sub: "100004", Name: "Linus Torvalds", Email: "linus@example.com", Color: "#f76707"},
	{Sub: "100005", Name: "Margaret Hamilton", Email: "margaret@example.com", Color: "#d6336c"},
	{Sub: "100006", Name: "Dennis Ritchie", Email: "dennis@example.com", Color: "#495057"},
}

var palette = []string{"#7048e8", "#1c7ed6", "#0ca678", "#f76707", "#d6336c", "#1098ad", "#ae3ec9"}

// Config configures the fake server.
type Config struct {
	// PublicURL is the browser facing base URL of the fake server, used to
	// build avatar URLs.
	PublicURL    string
	ClientID     string
	ClientSecret string
	Profiles     []Profile
	Now          func() time.Time
	// Random returns a random token; defaults to crypto/rand.
	Random func() string
}

type grant struct {
	profile     Profile
	redirectURI string
	expires     time.Time
}

// Server is the fake Google server.
type Server struct {
	cfg    Config
	mu     sync.Mutex
	codes  map[string]grant
	tokens map[string]Profile
	extra  map[string]Profile // custom accounts by sub
}

func randomToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// New returns a fake server.
func New(cfg Config) *Server {
	if cfg.Profiles == nil {
		cfg.Profiles = DefaultProfiles
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Random == nil {
		cfg.Random = randomToken
	}
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	return &Server{cfg: cfg, codes: map[string]grant{}, tokens: map[string]Profile{}, extra: map[string]Profile{}}
}

// Handler returns the HTTP handler with Google compatible paths.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /o/oauth2/v2/auth", s.authorize)
	mux.HandleFunc("POST /o/oauth2/v2/auth", s.approve)
	mux.HandleFunc("POST /token", s.token)
	mux.HandleFunc("GET /v1/userinfo", s.userinfo)
	mux.HandleFunc("GET /avatar/{sub}", s.avatar)
	return mux
}

type authRequest struct {
	ClientID    string
	RedirectURI string
	State       string
}

func (s *Server) parseAuthRequest(v url.Values) (authRequest, string) {
	ar := authRequest{ClientID: v.Get("client_id"), RedirectURI: v.Get("redirect_uri"), State: v.Get("state")}
	if ar.ClientID != s.cfg.ClientID {
		return ar, "unknown client_id"
	}
	u, err := url.Parse(ar.RedirectURI)
	if err != nil || !u.IsAbs() {
		return ar, "invalid redirect_uri"
	}
	return ar, ""
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ar, msg := s.parseAuthRequest(q)
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if q.Get("response_type") != "code" {
		http.Error(w, "unsupported response_type", http.StatusBadRequest)
		return
	}
	type card struct {
		Profile
		Avatar string
	}
	cards := make([]card, len(s.cfg.Profiles))
	for i, p := range s.cfg.Profiles {
		cards[i] = card{p, s.avatarURL(p.Sub)}
	}
	lang := language(r.Header.Get("Accept-Language"))
	var page bytes.Buffer
	data := map[string]any{"Request": ar, "Profiles": cards, "Lang": lang, "T": texts[lang]}
	if err := pages.ExecuteTemplate(&page, "chooser.html", data); err != nil {
		http.Error(w, "cannot render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = page.WriteTo(w) // a failed write means the client went away
}

// writeJSON writes v as JSON; an encoding error means the client went away.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ar, msg := s.parseAuthRequest(r.PostForm)
	if msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	redirect, _ := url.Parse(ar.RedirectURI)
	q := redirect.Query()
	q.Set("state", ar.State)
	if r.PostForm.Get("action") == "cancel" {
		q.Set("error", "access_denied")
		redirect.RawQuery = q.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusFound)
		return
	}
	p, ok := s.pick(r.PostForm)
	if !ok {
		http.Error(w, "unknown account", http.StatusBadRequest)
		return
	}
	code := s.cfg.Random()
	s.mu.Lock()
	s.codes[code] = grant{profile: p, redirectURI: ar.RedirectURI, expires: s.cfg.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	q.Set("code", code)
	redirect.RawQuery = q.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// pick resolves the account selected on the chooser page.
func (s *Server) pick(form url.Values) (Profile, bool) {
	if sub := form.Get("sub"); sub != "" {
		for _, p := range s.cfg.Profiles {
			if p.Sub == sub {
				return p, true
			}
		}
		return Profile{}, false
	}
	name := strings.TrimSpace(form.Get("name"))
	email := strings.ToLower(strings.TrimSpace(form.Get("email")))
	if name == "" || len(name) > 80 || !emailRe.MatchString(email) {
		return Profile{}, false
	}
	// Custom accounts get a stable subject derived from their email.
	sub := "custom-" + base64.RawURLEncoding.EncodeToString([]byte(email))
	p := Profile{Sub: sub, Name: name, Email: email, Color: palette[len(email)%len(palette)]}
	s.mu.Lock()
	s.extra[sub] = p
	s.mu.Unlock()
	return p, true
}

func tokenError(w http.ResponseWriter, code string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": code})
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, "invalid_request")
		return
	}
	f := r.PostForm
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = f.Get("client_id"), f.Get("client_secret")
	}
	if id != s.cfg.ClientID || secret != s.cfg.ClientSecret {
		tokenError(w, "invalid_client")
		return
	}
	if f.Get("grant_type") != "authorization_code" {
		tokenError(w, "unsupported_grant_type")
		return
	}
	s.mu.Lock()
	g, found := s.codes[f.Get("code")]
	delete(s.codes, f.Get("code"))
	s.mu.Unlock()
	if !found || s.cfg.Now().After(g.expires) || g.redirectURI != f.Get("redirect_uri") {
		tokenError(w, "invalid_grant")
		return
	}
	access := s.cfg.Random()
	s.mu.Lock()
	s.tokens[access] = g.profile
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        "openid email profile",
	})
}

func (s *Server) userinfo(w http.ResponseWriter, r *http.Request) {
	access, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	s.mu.Lock()
	p, found := s.tokens[access]
	s.mu.Unlock()
	if !ok || !found {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":            p.Sub,
		"email":          p.Email,
		"email_verified": true,
		"name":           p.Name,
		"picture":        s.avatarURL(p.Sub),
	})
}

func (s *Server) avatarURL(sub string) string {
	return s.cfg.PublicURL + "/avatar/" + url.PathEscape(sub)
}

func (s *Server) lookup(sub string) (Profile, bool) {
	for _, p := range s.cfg.Profiles {
		if p.Sub == sub {
			return p, true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.extra[sub]
	return p, ok
}

func (s *Server) avatar(w http.ResponseWriter, r *http.Request) {
	p, ok := s.lookup(r.PathValue("sub"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" viewBox="0 0 96 96">`+
		`<rect width="96" height="96" fill="%s"/>`+
		`<text x="48" y="50" dy=".35em" text-anchor="middle" font-family="Helvetica,Arial,sans-serif" font-size="38" font-weight="600" fill="#fff">%s</text></svg>`,
		template.HTMLEscapeString(p.Color), template.HTMLEscapeString(p.Initials()))
}
