package cli

import (
	"fmt"
	"io"
)

const usageText = `MongoDB Performance Troubleshooter (mpt)

Usage:
  mpt [--help]
  mpt [--version]

Options:
  -h, --help     Show help.
  --version      Show version.
`

const version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usageText)
		return 0
	}

	switch args[0] {
	case "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return 0
	case "--version":
		fmt.Fprintf(stdout, "mpt %s\n", version)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown argument: %s\n\n%s", args[0], usageText)
		return 1
	}
}
