package main

import (
	"os"

	"github.com/rostyslavpaliuha/mongodb-performance-troubleshooter/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
