// Package main provides the CLI entrypoint for the Talia tool.
package main

import (
	"os"

	"github.com/sustanza/talia"
)

var exitFunc = os.Exit

func main() {
	exitFunc(talia.RunCLI(os.Args[1:]))
}
