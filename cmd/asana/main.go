package main

import (
	"os"

	"github.com/jtsternberg/asana-cli/pkg/cmd"
)

func main() {
	code := cmd.Main()
	os.Exit(int(code))
}
