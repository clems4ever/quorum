package app

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runCommand(t *testing.T, ctx context.Context, environ map[string]string, args ...string) (string, error) {
	t.Helper()
	cmd := NewCommand("1.2.3", env(environ), quietDeps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(ctx)
	return out.String(), err
}

func TestCommandHelpAndVersion(t *testing.T) {
	out, err := runCommand(t, context.Background(), nil, "--version")
	if err != nil || !strings.Contains(out, "quorum version 1.2.3") {
		t.Fatalf("%q %v", out, err)
	}
	out, err = runCommand(t, context.Background(), nil, "serve", "--help")
	if err != nil || !strings.Contains(out, "--google-client-id") || !strings.Contains(out, "(env GOOGLE_CLIENT_ID)") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestServeRequiresCredentials(t *testing.T) {
	out, err := runCommand(t, context.Background(), nil, "serve")
	if err == nil || !strings.Contains(out, "Google client ID and secret are required") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestServeRejectsInvalidEnvironment(t *testing.T) {
	_, err := runCommand(t, context.Background(), map[string]string{"SESSION_TTL": "forever"}, "serve")
	if err == nil || !strings.Contains(err.Error(), "invalid SESSION_TTL") {
		t.Fatalf("got %v", err)
	}
}

func TestServeRuns(t *testing.T) {
	dir := t.TempDir()
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		// Flags and environment variables combine; flags win.
		_, err := runCommand(t, ctx,
			map[string]string{"GOOGLE_CLIENT_SECRET": "secret", "ADDR": "127.0.0.1:1", "STATIC_DIR": dir},
			"serve", "--addr", addr, "--google-client-id", "id", "--database", filepath.Join(dir, "q.db"),
			"--public-url", "https://q.test/")
		done <- err
	}()
	waitHealthy(t, addr)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeFailsOnBadDatabase(t *testing.T) {
	_, err := runCommand(t, context.Background(), nil, "serve",
		"--google-client-id", "id", "--google-client-secret", "s", "--database", filepath.Join(t.TempDir(), "no", "db"),
		"--session-ttl", time.Hour.String())
	if err == nil {
		t.Fatal("expected error")
	}
}
