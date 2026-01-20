package talia

import (
	"fmt"
	"os"
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

// spinnerFrames defines the animation frames for the terminal spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinner displays an animated spinner in the terminal during long operations.
type spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
}

// newSpinner creates a new spinner with the given message.
func newSpinner(message string) *spinner {
	return &spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a goroutine.
func (s *spinner) Start() {
	go func() {
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(os.Stderr, "\r\033[K") // Clear line
				close(s.done)
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %s", spinnerFrames[i%len(spinnerFrames)], s.message)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop halts the spinner animation and clears the line.
func (s *spinner) Stop() {
	close(s.stop)
	<-s.done
}

// progress tracks the current position in a series of operations.
type progress struct {
	current int
	total   int
}

// newProgress creates a new progress counter with the given total.
func newProgress(total int) *progress {
	return &progress{total: total}
}

// Increment advances the progress counter by one.
func (p *progress) Increment() {
	p.current++
}

// PrintCheck outputs a formatted progress line for a domain check.
func (p *progress) PrintCheck(domain string, available bool, reason AvailabilityReason) {
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
	fmt.Printf("[%d/%d] %s %s%s%s %s\n", p.current, p.total, domain, color, symbol, colorReset, status)
}

// checkStats tracks statistics for domain checks.
type checkStats struct {
	available int
	taken     int
	errors    int
	startTime time.Time
}

// newCheckStats creates a new stats tracker and records the start time.
func newCheckStats() *checkStats {
	return &checkStats{startTime: time.Now()}
}

// Record updates stats based on a check result.
func (s *checkStats) Record(available bool, reason AvailabilityReason) {
	switch {
	case reason == ReasonError:
		s.errors++
	case available:
		s.available++
	default:
		s.taken++
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
