// Package app wires the application together from its configuration.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"quorum/internal/api"
	"quorum/internal/auth"
	"quorum/internal/httpserver"
	"quorum/internal/static"
	"quorum/internal/store"
)

// Config is the runtime configuration (see NewCommand for its flags).
type Config struct {
	Addr               string
	PublicURL          string
	DatabasePath       string
	StaticDir          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleAuthURL      string
	GoogleTokenURL     string
	GoogleUserInfoURL  string
	SessionTTL         time.Duration
}

// Deps are the external dependencies of the application.
type Deps struct {
	Now        func() time.Time
	NewID      func() (string, error)
	Random     func(n int) (string, error)
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// DefaultDeps returns production dependencies.
func DefaultDeps() Deps {
	return Deps{
		Now:        time.Now,
		NewID:      func() (string, error) { return auth.RandomToken(9) },
		Random:     auth.RandomToken,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

// App is a ready to serve application.
type App struct {
	Handler http.Handler
	store   *store.SQLite
}

// Close releases resources.
func (a *App) Close() error { return a.store.Close() }

// New builds the application.
func New(ctx context.Context, c Config, d Deps) (*App, error) {
	db, err := store.Open(ctx, c.DatabasePath)
	if err != nil {
		return nil, err
	}
	sessions := &auth.Sessions{
		Store:  db,
		TTL:    c.SessionTTL,
		Secure: strings.HasPrefix(c.PublicURL, "https://"),
		Now:    d.Now,
		Random: d.Random,
	}
	provider := auth.NewGoogle(auth.GoogleConfig{
		ClientID:     c.GoogleClientID,
		ClientSecret: c.GoogleClientSecret,
		RedirectURL:  c.PublicURL + "/auth/callback",
		AuthURL:      c.GoogleAuthURL,
		TokenURL:     c.GoogleTokenURL,
		UserInfoURL:  c.GoogleUserInfoURL,
		HTTPClient:   d.HTTPClient,
	})
	mux := http.NewServeMux()
	(&auth.Handler{Provider: provider, Sessions: sessions, Users: db, Logger: d.Logger}).Register(mux)
	(&api.API{Store: db, Auth: sessions, Now: d.Now, NewID: d.NewID, Logger: d.Logger}).Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok") // a failed write means the client went away
	})
	mux.Handle("/", static.Handler(os.DirFS(c.StaticDir)))
	return &App{Handler: logRequests(d.Logger, d.Now, securityHeaders(mux)), store: db}, nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// logRequests logs API and auth requests (static assets are skipped).
func logRequests(logger *slog.Logger, now func() time.Time, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		start := now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", now().Sub(start))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// Run builds and serves the application until ctx is canceled.
func Run(ctx context.Context, c Config, d Deps) (err error) {
	a, err := New(ctx, c, d)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, a.Close()) }()
	srv := &http.Server{Addr: c.Addr, Handler: a.Handler, ReadHeaderTimeout: 10 * time.Second}
	return httpserver.Serve(ctx, srv, d.Logger)
}
