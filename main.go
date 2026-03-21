package main

import (
	"os"

	"builder/cs-builder/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
