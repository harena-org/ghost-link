package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONMode controls whether commands produce JSON output on stdout.
// Set from the --json global flag in main.go.
var JSONMode bool

// PrintResult outputs the command result. In JSON mode, it marshals data
// to stdout. Otherwise, it calls humanFn to produce human-readable output.
func PrintResult(data interface{}, humanFn func()) {
	if JSONMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)
		return
	}
	humanFn()
}

// PrintError outputs an error. In JSON mode, it writes {"error":"msg"}
// to stdout. Otherwise, it writes to stderr.
func PrintError(err error) {
	if JSONMode {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(map[string]string{"error": err.Error()})
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
}

// Status prints a progress message to stderr (visible in both modes).
func Status(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// Statusf prints a formatted progress message to stderr.
func Statusf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
