// Command quorum runs the Quorum web application.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"quorum/internal/app"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.NewCommand(version, os.Getenv, app.DefaultDeps).ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}
