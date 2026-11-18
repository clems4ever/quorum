// Package auth implements Google sign-in and cookie based sessions.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"

	"quorum/internal/model"
)

// Default Google endpoints.
const (
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token" //nolint:gosec // G101: a public URL, not a credential
	GoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// Provider is an OAuth2 identity provider.
type Provider interface {
	// AuthCodeURL returns the URL the browser is sent to for signing in.
	AuthCodeURL(state string) string
	// Exchange trades an authorization code for the signed-in user.
	Exchange(ctx context.Context, code string) (model.User, error)
}

// GoogleConfig configures a Google OIDC provider. Endpoint URLs default to
// the real Google endpoints when empty.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	// HTTPClient is used for server-to-server calls; defaults to http.DefaultClient.
	HTTPClient *http.Client
}

// Google signs users in with Google (or any compatible OIDC server).
type Google struct {
	oauth       *oauth2.Config
	userInfoURL string
	client      *http.Client
}

// NewGoogle builds a Google provider.
func NewGoogle(c GoogleConfig) *Google {
	or := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Google{
		oauth: &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:   or(c.AuthURL, GoogleAuthURL),
				TokenURL:  or(c.TokenURL, GoogleTokenURL),
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		userInfoURL: or(c.UserInfoURL, GoogleUserInfoURL),
		client:      client,
	}
}

// AuthCodeURL implements Provider.
func (g *Google) AuthCodeURL(state string) string {
	return g.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "select_account"))
}

type userInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Exchange implements Provider.
func (g *Google) Exchange(ctx context.Context, code string) (model.User, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.client)
	tok, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return model.User{}, fmt.Errorf("exchange code: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.userInfoURL, nil)
	if err != nil {
		return model.User{}, fmt.Errorf("fetch userinfo: %w", err)
	}
	resp, err := g.oauth.Client(ctx, tok).Do(req)
	if err != nil {
		return model.User{}, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only body; a close error is irrelevant
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return model.User{}, fmt.Errorf("fetch userinfo: status %d: %s", resp.StatusCode, body)
	}
	var info userInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return model.User{}, fmt.Errorf("decode userinfo: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return model.User{}, fmt.Errorf("userinfo is missing subject or email")
	}
	if !info.EmailVerified {
		return model.User{}, fmt.Errorf("email %s is not verified", info.Email)
	}
	name := info.Name
	if name == "" {
		name = info.Email
	}
	return model.User{ID: "google:" + info.Sub, Email: info.Email, Name: name, Picture: info.Picture}, nil
}
