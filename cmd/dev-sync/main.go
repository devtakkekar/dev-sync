package main

import (
	"os"

	"github.com/devsync/dev-sync/internal/cli"
)

func main() {
	exitCode := cli.Run(os.Args[1:])
	os.Exit(exitCode)
}
