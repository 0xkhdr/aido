// Command aido is the aido-core CLI. It is wiring only: every decision about
// what a config means lives in internal/config (structure.md S5, design.md I6).
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches a subcommand and returns the process exit code. It is
// separated from main so tests can drive it without a subprocess.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "config":
		if len(args) < 2 || args[1] != "show" {
			usage(stderr)
			return 2
		}
		return configShow(args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "aido: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, "usage: aido config show [project-dir]\n")
}
