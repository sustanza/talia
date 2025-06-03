package main

import (
	"os"
	"testing"
)

func TestMainExit(t *testing.T) {
	defer func() { exitFunc = os.Exit }()
	var got int
	exitFunc = func(code int) { got = code }
	oldArgs := os.Args
	os.Args = []string{"talia"}
	defer func() { os.Args = oldArgs }()
	main()
	if got == 0 {
		t.Errorf("expected non-zero exit code")
	}
}
