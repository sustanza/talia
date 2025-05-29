package main

import (
	"os"

	"github.com/sustanza/talia"
)

func main() {
	os.Exit(talia.RunCLI(os.Args[1:]))
}
