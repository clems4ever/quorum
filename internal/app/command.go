package app

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"quorum/internal/envflag"
)

var serveBindings = []envflag.Binding{
	{Flag: "addr", Env: "ADDR"},
	{Flag: "public-url", Env: "PUBLIC_URL"},
	{Flag: "database", Env: "DATABASE_PATH"},
	{Flag: "static-dir", Env: "STATIC_DIR"},
	{Flag: "google-client-id", Env: "GOOGLE_CLIENT_ID"},
	{Flag: "google-client-secret", Env: "GOOGLE_CLIENT_SECRET"},
	{Flag: "google-auth-url", Env: "GOOGLE_AUTH_URL"},
	{Flag: "google-token-url", Env: "GOOGLE_TOKEN_URL"},
	{Flag: "google-userinfo-url", Env: "GOOGLE_USERINFO_URL"},
	{Flag: "session-ttl", Env: "SESSION_TTL"},
}

// NewCommand returns the "quorum" command line interface. getenv provides
// environment fallbacks for flags and deps builds the runtime dependencies.
func NewCommand(version string, getenv func(string) string, deps func() Deps) *cobra.Command {
	root := &cobra.Command{
		Use:          "quorum",
		Short:        "Quorum finds the time that works for everyone",
		Version:      version,
		SilenceUsage: true,
	}
	root.AddCommand(newServeCommand(getenv, deps))
	return root
}

func newServeCommand(getenv func(string) string, deps func() Deps) *cobra.Command {
	var c Config
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the web server",
		Long: "Run the web server.\n\nEvery flag can also be set with the environment variable shown in its " +
			"description; flags take precedence. Prefer the environment for secrets.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := envflag.Apply(cmd.Flags(), getenv, serveBindings); err != nil {
				return err
			}
			if c.GoogleClientID == "" || c.GoogleClientSecret == "" {
				return errors.New("a Google client ID and secret are required (--google-client-id/--google-client-secret)")
			}
			c.PublicURL = strings.TrimRight(c.PublicURL, "/")
			return Run(cmd.Context(), c, deps())
		},
	}
	f := cmd.Flags()
	f.StringVar(&c.Addr, "addr", ":8080", envflag.Usage("listen address", "ADDR"))
	f.StringVar(&c.PublicURL, "public-url", "http://localhost:8080",
		envflag.Usage("external URL of the app; the OAuth redirect is <public-url>/auth/callback", "PUBLIC_URL"))
	f.StringVar(&c.DatabasePath, "database", "quorum.db", envflag.Usage("SQLite database file", "DATABASE_PATH"))
	f.StringVar(&c.StaticDir, "static-dir", "web/dist", envflag.Usage("directory of the built web client", "STATIC_DIR"))
	f.StringVar(&c.GoogleClientID, "google-client-id", "", envflag.Usage("Google OAuth client ID", "GOOGLE_CLIENT_ID"))
	f.StringVar(&c.GoogleClientSecret, "google-client-secret", "",
		envflag.Usage("Google OAuth client secret", "GOOGLE_CLIENT_SECRET"))
	f.StringVar(&c.GoogleAuthURL, "google-auth-url", "",
		envflag.Usage("override the Google authorization endpoint", "GOOGLE_AUTH_URL"))
	f.StringVar(&c.GoogleTokenURL, "google-token-url", "",
		envflag.Usage("override the Google token endpoint", "GOOGLE_TOKEN_URL"))
	f.StringVar(&c.GoogleUserInfoURL, "google-userinfo-url", "",
		envflag.Usage("override the Google userinfo endpoint", "GOOGLE_USERINFO_URL"))
	f.DurationVar(&c.SessionTTL, "session-ttl", 30*24*time.Hour, envflag.Usage("session lifetime", "SESSION_TTL"))
	return cmd
}
