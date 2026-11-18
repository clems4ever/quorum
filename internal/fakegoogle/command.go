package fakegoogle

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"quorum/internal/envflag"
)

var bindings = []envflag.Binding{
	{Flag: "addr", Env: "ADDR"},
	{Flag: "prefix", Env: "PREFIX"},
	{Flag: "upstream", Env: "UPSTREAM"},
	{Flag: "public-url", Env: "PUBLIC_URL"},
	{Flag: "client-id", Env: "CLIENT_ID"},
	{Flag: "client-secret", Env: "CLIENT_SECRET"},
}

// Serve runs srv until ctx is canceled.
type Serve func(ctx context.Context, srv *http.Server, logger *slog.Logger) error

// NewCommand returns the "fakegoogle" command line interface.
func NewCommand(getenv func(string) string, serve Serve, logger *slog.Logger) *cobra.Command {
	var (
		cfg              Config
		addr, prefix, up string
	)
	cmd := &cobra.Command{
		Use:   "fakegoogle",
		Short: "Run a fake Google OAuth provider for demos (never use in production)",
		Long: "Run a fake Google OAuth provider: users sign in by clicking a predefined profile.\n\n" +
			"With --prefix and --upstream the provider and the app share a single origin.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := envflag.Apply(cmd.Flags(), getenv, bindings); err != nil {
				return err
			}
			if cfg.PublicURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
				return errors.New("--public-url, --client-id and --client-secret are required")
			}
			var upstream *url.URL
			if up != "" {
				u, err := url.Parse(up)
				if err != nil || !u.IsAbs() {
					return errors.New("--upstream must be an absolute URL")
				}
				upstream = u
			}
			srv := &http.Server{
				Addr:              addr,
				Handler:           Gateway(strings.TrimRight(prefix, "/"), New(cfg).Handler(), upstream),
				ReadHeaderTimeout: 10 * time.Second,
			}
			return serve(cmd.Context(), srv, logger)
		},
	}
	f := cmd.Flags()
	f.StringVar(&addr, "addr", ":9090", envflag.Usage("listen address", "ADDR"))
	f.StringVar(&prefix, "prefix", "", envflag.Usage("path prefix of the fake endpoints, e.g. /fake-google", "PREFIX"))
	f.StringVar(&up, "upstream", "", envflag.Usage("URL receiving every request outside the prefix", "UPSTREAM"))
	f.StringVar(&cfg.PublicURL, "public-url", "",
		envflag.Usage("browser facing URL of the fake provider (including the prefix)", "PUBLIC_URL"))
	f.StringVar(&cfg.ClientID, "client-id", "", envflag.Usage("accepted OAuth client ID", "CLIENT_ID"))
	f.StringVar(&cfg.ClientSecret, "client-secret", "", envflag.Usage("accepted OAuth client secret", "CLIENT_SECRET"))
	return cmd
}
