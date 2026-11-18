package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"quorum/internal/model"
)

// SessionCookie is the name of the cookie holding the session token.
const SessionCookie = "quorum_session"

// SessionStore persists sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, tokenHash, userID string, expires time.Time) error
	SessionUser(ctx context.Context, tokenHash string, now time.Time) (model.User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
}

// Sessions issues and resolves session cookies.
type Sessions struct {
	Store  SessionStore
	TTL    time.Duration
	Secure bool
	Now    func() time.Time
	Random func(n int) (string, error)
}

// RandomToken returns n random bytes encoded as URL-safe base64.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Start creates a session for the user and sets the cookie.
func (s *Sessions) Start(ctx context.Context, w http.ResponseWriter, userID string) error {
	token, err := s.Random(32)
	if err != nil {
		return err
	}
	expires := s.Now().Add(s.TTL)
	if err := s.Store.CreateSession(ctx, hashToken(token), userID, expires); err != nil {
		return err
	}
	c := s.cookie(SessionCookie, token, "/")
	c.Expires = expires
	http.SetCookie(w, c)
	return nil
}

// cookie returns an HttpOnly, SameSite=Lax cookie that is Secure whenever the
// app is served over HTTPS.
func (s *Sessions) cookie(name, value, path string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   s.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expired returns a cookie that deletes name from the browser.
func (s *Sessions) expired(name, path string) *http.Cookie {
	c := s.cookie(name, "", path)
	c.MaxAge = -1
	return c
}

// User returns the user of the request's session, or model.ErrNotFound.
func (s *Sessions) User(r *http.Request) (model.User, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c.Value == "" {
		return model.User{}, model.ErrNotFound
	}
	return s.Store.SessionUser(r.Context(), hashToken(c.Value), s.Now())
}

// End deletes the request's session and clears the cookie.
func (s *Sessions) End(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, s.expired(SessionCookie, "/"))
	c, err := r.Cookie(SessionCookie)
	if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
		return nil
	}
	return s.Store.DeleteSession(r.Context(), hashToken(c.Value))
}
