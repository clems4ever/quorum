package envflag

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("addr", ":8080", "")
	fs.Duration("ttl", time.Hour, "")
	return fs
}

func TestApply(t *testing.T) {
	bindings := []Binding{{"addr", "ADDR"}, {"ttl", "TTL"}}

	fs := flags()
	if err := fs.Parse([]string{"--addr", ":1"}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(fs, env(map[string]string{"ADDR": ":2", "TTL": "5m"}), bindings); err != nil {
		t.Fatal(err)
	}
	addr, _ := fs.GetString("addr")
	ttl, _ := fs.GetDuration("ttl")
	if addr != ":1" || ttl != 5*time.Minute {
		t.Fatalf("flag should win over env, env over default: %q %v", addr, ttl)
	}

	fs = flags()
	if err := Apply(fs, env(nil), bindings); err != nil {
		t.Fatal(err)
	}
	if addr, _ := fs.GetString("addr"); addr != ":8080" {
		t.Fatalf("default expected, got %q", addr)
	}
}

func TestApplyErrors(t *testing.T) {
	err := Apply(flags(), env(map[string]string{"TTL": "soon"}), []Binding{{"ttl", "TTL"}})
	if err == nil || !strings.Contains(err.Error(), "invalid TTL") {
		t.Fatalf("got %v", err)
	}
	if err := Apply(flags(), env(nil), []Binding{{"nope", "NOPE"}}); err == nil {
		t.Fatal("expected unknown flag error")
	}
}

func TestUsage(t *testing.T) {
	if got := Usage("Listen address", "ADDR"); got != "Listen address (env ADDR)" {
		t.Fatal(got)
	}
}
