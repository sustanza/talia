package talia

import (
	"fmt"
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

// Status symbols for progress output.
const (
	symbolAvailable = "✓"
	symbolTaken     = "✗"
	symbolError     = "⚠"
)

// progress tracks the current position in a series of operations (thread-safe).
type progress struct {
	current int64
	total   int64
	mu      sync.Mutex // protects printing
}

// newProgress creates a new progress counter with the given total.
func newProgress(total int) *progress {
	return &progress{total: int64(total)}
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
	fmt.Printf("[%d/%d] %s %s%s%s %s\n", current, p.total, domain, color, symbol, colorReset, status)
	p.mu.Unlock()
}

// checkStats tracks statistics for domain checks (thread-safe).
type checkStats struct {
	available int64
	taken     int64
	errors    int64
	startTime time.Time
}

// newCheckStats creates a new stats tracker and records the start time.
func newCheckStats() *checkStats {
	return &checkStats{startTime: time.Now()}
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
		fmt.Printf("  %s%s %d available%s\n", colorGreen, symbolAvailable, s.available, colorReset)
	}
	if s.taken > 0 {
		fmt.Printf("  %s%s %d taken%s\n", colorRed, symbolTaken, s.taken, colorReset)
	}
	if s.errors > 0 {
		fmt.Printf("  %s%s %d errors%s\n", colorYellow, symbolError, s.errors, colorReset)
	}
}
