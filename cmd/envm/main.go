package main

import (
	"os"

	"martinshaw.co/environment-manager/internal/cli"
)

var version = "0.1.0"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
