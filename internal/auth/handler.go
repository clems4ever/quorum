package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"quorum/internal/model"
)

const stateCookie = "quorum_oauth_state"

// UserSaver persists users that signed in.
type UserSaver interface {
	UpsertUser(ctx context.Context, u model.User) error
}

// Handler serves the /auth/* routes.
type Handler struct {
	Provider Provider
	Sessions *Sessions
	Users    UserSaver
	Logger   *slog.Logger
}

// Register mounts the routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", h.login)
	mux.HandleFunc("GET /auth/callback", h.callback)
	mux.HandleFunc("POST /auth/logout", h.logout)
}

// safeNext only allows local absolute paths to avoid open redirects.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := h.Sessions.Random(24)
	if err != nil {
		h.fail(w, "generate state", err)
		return
	}
	c := h.Sessions.cookie(stateCookie, state+"|"+safeNext(r.URL.Query().Get("next")), "/auth/")
	c.Expires = h.Sessions.Now().Add(10 * time.Minute)
	http.SetCookie(w, c)
	http.Redirect(w, r, h.Provider.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(stateCookie)
	if err != nil {
		http.Error(w, "missing sign-in state, please try again", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, h.Sessions.expired(stateCookie, "/auth/"))
	state, next, _ := strings.Cut(c.Value, "|")
	q := r.URL.Query()
	if state == "" || q.Get("state") != state {
		http.Error(w, "invalid sign-in state, please try again", http.StatusBadRequest)
		return
	}
	if q.Get("error") != "" {
		// The user declined (or Google failed): go back, still signed out.
		http.Redirect(w, r, safeNext(next), http.StatusFound)
		return
	}
	user, err := h.Provider.Exchange(r.Context(), q.Get("code"))
	if err != nil {
		h.Logger.Warn("sign-in failed", "err", err)
		http.Error(w, "sign-in failed", http.StatusUnauthorized)
		return
	}
	if err := h.Users.UpsertUser(r.Context(), user); err != nil {
		h.fail(w, "save user", err)
		return
	}
	if err := h.Sessions.Start(r.Context(), w, user.ID); err != nil {
		h.fail(w, "start session", err)
		return
	}
	http.Redirect(w, r, safeNext(next), http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Sessions.End(w, r); err != nil {
		h.fail(w, "end session", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, what string, err error) {
	h.Logger.Error(what, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
