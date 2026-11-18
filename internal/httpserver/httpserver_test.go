package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestServeAndShutdown(t *testing.T) {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	srv := &http.Server{Addr: addr, Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	go func() { done <- Serve(ctx, srv, logger) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeListenError(t *testing.T) {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Error(err)
		}
	}()
	srv := &http.Server{Addr: l.Addr().String(), ReadHeaderTimeout: time.Second}
	if err := Serve(context.Background(), srv, logger); err == nil {
		t.Fatal("expected address in use error")
	}
}
