package main

import (
	"os"
	"strings"

	"github.com/sustanza/talia"
)

var exitFunc = os.Exit

func loadEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			_ = os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
}

func main() {
	loadEnv()
	exitFunc(talia.RunCLI(os.Args[1:]))
}
