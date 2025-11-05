package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// EnvPrefix is the default prefix applied when resolving environment variables for flags.
// Override by providing a different prefix in ApplyEnvOverrides if needed.
const EnvPrefix = "KUBEMIN"

// ApplyEnvOverrides walks the provided FlagSet and, for every flag that wasn't set via CLI,
// attempts to read an environment variable matching the flag name.
// Flag names are transformed by converting to upper snake case and prepending the provided prefix.
// For example, the flag "bind-addr" becomes "KUBEMIN_BIND_ADDR".
func ApplyEnvOverrides(fs *pflag.FlagSet, prefix string) error {
	var errs []error
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			return
		}
		envKey := buildEnvKey(prefix, f.Name)
		if val, ok := os.LookupEnv(envKey); ok {
			if err := fs.Set(f.Name, val); err != nil {
				errs = append(errs, fmt.Errorf("apply %s to flag --%s: %w", envKey, f.Name, err))
			}
		}
	})
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func buildEnvKey(prefix, name string) string {
	canonical := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if strings.TrimSpace(prefix) == "" {
		return canonical
	}
	return strings.ToUpper(prefix) + "_" + canonical
}
