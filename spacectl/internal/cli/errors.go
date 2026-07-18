package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// errNoServer is returned when no server is configured anywhere.
var errNoServer = errors.New("no server configured — pass --server, set $SPACECTL_SERVER, or run 'spacectl login --server <url>'")

// usageError marks errors that should exit with code 2 (usage).
type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// IsUsageError reports whether err (or anything it wraps) is a usage error.
func IsUsageError(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}

// exactArgs is cobra.ExactArgs but yields usage errors (exit code 2).
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return &usageError{err: fmt.Errorf("%q requires exactly %d argument(s), received %d", cmd.CommandPath(), n, len(args))}
		}
		return nil
	}
}

// noArgs rejects positional arguments with a usage error.
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return &usageError{err: fmt.Errorf("%q accepts no arguments, received %d", cmd.CommandPath(), len(args))}
	}
	return nil
}
