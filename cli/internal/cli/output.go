package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	outputTable = ""
	outputJSON  = "json"
	outputYAML  = "yaml"
)

// addOutputFlag registers --output/-o on cmd.
func addOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", "", "output format: json|yaml (default: table)")
}

// outputFormat validates and returns the --output value.
func outputFormat(cmd *cobra.Command) (string, error) {
	format, _ := cmd.Flags().GetString("output")
	switch format {
	case outputTable, outputJSON, outputYAML:
		return format, nil
	default:
		return "", &usageError{err: fmt.Errorf("invalid --output %q (expected json or yaml)", format)}
	}
}

// printEncoded renders v as JSON or YAML.
func printEncoded(w io.Writer, format string, v any) error {
	switch format {
	case outputJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case outputYAML:
		data, err := yaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// newTable returns a tabwriter suitable for aligned column output.
func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 8, 3, ' ', 0)
}

// formatAge renders a compact k8s-style age (e.g. 42s, 5m, 3h, 2d).
func formatAge(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// dash substitutes "-" for empty table cells.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
