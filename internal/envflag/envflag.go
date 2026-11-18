// Package envflag lets command line flags fall back to environment variables.
package envflag

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Binding associates a flag with an environment variable.
type Binding struct {
	Flag string
	Env  string
}

// Apply sets every flag that was not given on the command line from its
// environment variable, when that variable is not empty. Flags therefore take
// precedence over the environment, which takes precedence over defaults.
func Apply(flags *pflag.FlagSet, getenv func(string) string, bindings []Binding) error {
	for _, b := range bindings {
		f := flags.Lookup(b.Flag)
		if f == nil {
			return fmt.Errorf("unknown flag %q", b.Flag)
		}
		if f.Changed {
			continue
		}
		if v := getenv(b.Env); v != "" {
			if err := flags.Set(b.Flag, v); err != nil {
				return fmt.Errorf("invalid %s: %w", b.Env, err)
			}
		}
	}
	return nil
}

// Usage returns a help suffix naming the environment variable of a flag.
func Usage(text, env string) string {
	return fmt.Sprintf("%s (env %s)", text, env)
}
