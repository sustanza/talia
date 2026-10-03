package talia

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCheckOutput runs a WHOIS check of two domains and returns what was printed to stdout.
func runCheckOutput(t *testing.T) string {
	t.Helper()
	addr, _ := startCountingWhois(t)
	file := filepath.Join(t.TempDir(), "domains.json")
	if err := os.WriteFile(file, []byte(`[{"domain":"a.com"},{"domain":"b.com"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TALIA_LIGHTSPEED", "")
	stdout, _ := captureOutput(t, func() {
		if code := RunCLI([]string{"--whois=" + addr, "--sleep=0", file}); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	return stdout
}

// simulateTerminal makes Talia treat stdout as a terminal for the rest of the test.
func simulateTerminal(t *testing.T) {
	t.Helper()
	old := stdoutIsTerminal
	stdoutIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdoutIsTerminal = old })
}

func TestProgressOutput_NoColorWhenPiped(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	stdout := runCheckOutput(t)

	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("piped output contains ANSI escapes: %q", stdout)
	}
	if !strings.Contains(stdout, "a.com ✓ available") || !strings.Contains(stdout, "✓ 2 available") {
		t.Errorf("piped output lost its symbols or text: %q", stdout)
	}
}

func TestProgressOutput_ColorOnTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	simulateTerminal(t)

	stdout := runCheckOutput(t)

	if !strings.Contains(stdout, "a.com \x1b[32m✓\x1b[0m available") {
		t.Errorf("terminal output should color progress lines: %q", stdout)
	}
}

func TestProgressOutput_NoColorEnvDisablesColorOnTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	simulateTerminal(t)

	stdout := runCheckOutput(t)

	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("NO_COLOR output contains ANSI escapes: %q", stdout)
	}
}
