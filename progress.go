package talia

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ANSI color codes for terminal output.
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

// stdoutIsTerminal reports whether stdout is a terminal. Tests replace it to simulate one.
var stdoutIsTerminal = func() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// colorEnabled reports whether output should use ANSI colors: only on a terminal,
// and never when NO_COLOR is set to a non-empty value (https://no-color.org).
func colorEnabled() bool {
	return os.Getenv("NO_COLOR") == "" && stdoutIsTerminal()
}

// colorize wraps text in the given color when enabled.
func colorize(enabled bool, color, text string) string {
	if !enabled {
		return text
	}
	return color + text + colorReset
}

// Status symbols for progress output.
const (
	symbolAvailable = "✓"
	symbolTaken     = "✗"
	symbolError     = "⚠"
)

// progress tracks the current position in a series of operations (thread-safe).
type progress struct {
	current  int64
	total    int64
	useColor bool
	mu       sync.Mutex // protects printing
}

// newProgress creates a new progress counter with the given total.
func newProgress(total int) *progress {
	return &progress{total: int64(total), useColor: colorEnabled()}
}

// IncrementAndPrint atomically increments the counter and prints the check result.
// This is thread-safe for concurrent use.
func (p *progress) IncrementAndPrint(domain string, available bool, reason AvailabilityReason) {
	current := atomic.AddInt64(&p.current, 1)

	var symbol, color, status string
	switch {
	case reason == ReasonError:
		symbol = symbolError
		color = colorYellow
		status = "error"
	case available:
		symbol = symbolAvailable
		color = colorGreen
		status = "available"
	default:
		symbol = symbolTaken
		color = colorRed
		status = "taken"
	}

	p.mu.Lock()
	fmt.Printf("[%d/%d] %s %s %s\n", current, p.total, domain, colorize(p.useColor, color, symbol), status)
	p.mu.Unlock()
}

// checkStats tracks statistics for domain checks (thread-safe).
type checkStats struct {
	available int64
	taken     int64
	errors    int64
	startTime time.Time
	useColor  bool
}

// newCheckStats creates a new stats tracker and records the start time.
func newCheckStats() *checkStats {
	return &checkStats{startTime: time.Now(), useColor: colorEnabled()}
}

// Record updates stats based on a check result (thread-safe).
func (s *checkStats) Record(available bool, reason AvailabilityReason) {
	switch {
	case reason == ReasonError:
		atomic.AddInt64(&s.errors, 1)
	case available:
		atomic.AddInt64(&s.available, 1)
	default:
		atomic.AddInt64(&s.taken, 1)
	}
}

// PrintSummary outputs a summary of the check results.
func (s *checkStats) PrintSummary() {
	elapsed := time.Since(s.startTime)
	fmt.Printf("\nDone in %.1fs\n", elapsed.Seconds())
	if s.available > 0 {
		fmt.Printf("  %s\n", colorize(s.useColor, colorGreen, fmt.Sprintf("%s %d available", symbolAvailable, s.available)))
	}
	if s.taken > 0 {
		fmt.Printf("  %s\n", colorize(s.useColor, colorRed, fmt.Sprintf("%s %d taken", symbolTaken, s.taken)))
	}
	if s.errors > 0 {
		fmt.Printf("  %s\n", colorize(s.useColor, colorYellow, fmt.Sprintf("%s %d errors", symbolError, s.errors)))
	}
}
