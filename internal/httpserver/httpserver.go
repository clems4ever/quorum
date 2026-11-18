// Package httpserver runs HTTP servers with graceful shutdown.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Serve runs srv until ctx is canceled, then shuts it down gracefully.
func Serve(ctx context.Context, srv *http.Server, logger *slog.Logger) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	logger.Info("listening", "addr", srv.Addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
