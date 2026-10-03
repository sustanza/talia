package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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

// runMainInDir runs main() in dir with the given args and returns its stderr output.
func runMainInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	t.Chdir(dir)
	t.Setenv("WHOIS_SERVER", "127.0.0.1:1")
	t.Setenv("TALIA_SUGGEST", "")

	oldArgs, oldStderr := os.Args, os.Stderr
	defer func() { os.Args, os.Stderr, exitFunc = oldArgs, oldStderr, os.Exit }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	os.Args = append([]string{"talia"}, args...)
	exitFunc = func(int) {}

	main()

	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return dir
}

func TestMainShellEnvOverridesDotEnv(t *testing.T) {
	dir := writeDotEnv(t, "TALIA_FILE=fromdotenv.json\n")
	t.Setenv("TALIA_FILE", "fromshell.json")

	stderr := runMainInDir(t, dir)

	if !strings.Contains(stderr, "Error reading fromshell.json") {
		t.Errorf("expected shell TALIA_FILE to win, stderr: %q", stderr)
	}
}

func TestMainDotEnvStripsQuotes(t *testing.T) {
	for _, line := range []string{`TALIA_FILE="quoted.json"`, `TALIA_FILE='quoted.json'`} {
		t.Run(line, func(t *testing.T) {
			dir := writeDotEnv(t, line+"\n")
			// t.Setenv registers restoration of the original value; the unset then
			// leaves TALIA_FILE absent so the .env value applies.
			t.Setenv("TALIA_FILE", "")
			_ = os.Unsetenv("TALIA_FILE")

			stderr := runMainInDir(t, dir)

			if !strings.Contains(stderr, "Error reading quoted.json:") {
				t.Errorf("expected quotes stripped from .env value, stderr: %q", stderr)
			}
		})
	}
}
