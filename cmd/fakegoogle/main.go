// Command fakegoogle runs a fake Google OAuth2 provider for demos: users sign
// in by clicking one of the predefined profiles. Never use it in production.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"quorum/internal/fakegoogle"
	"quorum/internal/httpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	err := fakegoogle.NewCommand(os.Getenv, httpserver.Serve, logger).ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}
