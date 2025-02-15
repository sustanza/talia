// Package main_test contains tests for Talia, a CLI for WHOIS-based domain checks.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// captureOutput redirects stdout/stderr to pipes, runs fn, then returns the captured strings.
func captureOutput(t *testing.T, fn func()) (string, string) {
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	os.Stdout = wOut
	os.Stderr = wErr

	outCh := make(chan string)
	errCh := make(chan string)

	// Copy from rOut and rErr in goroutines.
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	fn()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return <-outCh, <-errCh
}

// TestCheckDomainAvailability validates the WHOIS-checking function alone.
func TestCheckDomainAvailability(t *testing.T) {
	tests := []struct {
		name          string
		serverHandler func(conn net.Conn)
		expectedAvail bool
		expectedErr   bool
	}{
		{
			name: "Domain not found (indicating availability)",
			serverHandler: func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				io.Copy(io.Discard, conn) // discard anything client sends
				io.WriteString(conn, "No match for example.com\n")
				time.Sleep(50 * time.Millisecond)
			},
			expectedAvail: true,
			expectedErr:   false,
		},
		{
			name: "Domain found (indicating not available)",
			serverHandler: func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				io.Copy(io.Discard, conn)
				io.WriteString(conn, "Some registration data\nDomain Name: example.com\n")
				time.Sleep(50 * time.Millisecond)
			},
			expectedAvail: false,
			expectedErr:   false,
		},
		{
			name: "Immediate connection close (should produce read error)",
			serverHandler: func(conn net.Conn) {
				conn.Close()
			},
			expectedAvail: false,
			expectedErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("Failed to listen on port: %v", err)
			}
			defer ln.Close()

			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				tc.serverHandler(conn)
			}()

			available, _, err := checkDomainAvailability("example.com", ln.Addr().String())
			if tc.expectedErr && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tc.expectedErr && err != nil {
				t.Errorf("Did not expect an error but got: %v", err)
			}
			if available != tc.expectedAvail {
				t.Errorf("Expected available=%v, got %v", tc.expectedAvail, available)
			}
		})
	}
}

// TestMainFunction checks the entire flow with a mocked WHOIS server, a real JSON file,
// and verifies that runCLI updates the file as expected.
func TestMainFunction(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainFunction", flag.ContinueOnError)

	// Create a temporary JSON file with sample domains.
	tmpFile, err := os.CreateTemp("", "domains_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	domainData := []DomainRecord{
		{Domain: "example1.com"},
		{Domain: "example2.com"},
	}
	inputJSON, _ := json.MarshalIndent(domainData, "", "  ")
	if _, err := tmpFile.Write(inputJSON); err != nil {
		t.Fatalf("Failed to write JSON: %v", err)
	}
	tmpFile.Close()

	// Start a mock WHOIS server that always responds "No match for".
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func(c net.Conn) {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				io.Copy(io.Discard, c)
				io.WriteString(c, "No match for domain\n")
				time.Sleep(50 * time.Millisecond)
			}(conn)
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			"--sleep=1s", // override default to speed up test
			tmpFile.Name(),
		})
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}
	})

	updatedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	var updatedRecords []DomainRecord
	if err := json.Unmarshal(updatedContent, &updatedRecords); err != nil {
		t.Fatalf("Error unmarshaling JSON: %v", err)
	}

	if len(updatedRecords) != 2 {
		t.Errorf("Expected 2 records, got %d", len(updatedRecords))
	}
	for _, rec := range updatedRecords {
		if !rec.Available {
			t.Errorf("Expected domain %s to be available, but it was not", rec.Domain)
		}
		if !strings.Contains(rec.Log, "No match for domain") {
			t.Errorf("Expected Log to contain 'No match for domain', got: %s", rec.Log)
		}
	}
}

// TestMainInvalidArgs ensures we fail with no arguments.
func TestMainInvalidArgs(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainInvalidArgs", flag.ContinueOnError)

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{}) // no arguments at all
		if code == 0 {
			t.Errorf("Expected non-zero exit code with missing arguments")
		}
	})
}

// TestMainBadFile ensures we fail if the provided file is actually a directory.
func TestMainBadFile(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainBadFile", flag.ContinueOnError)

	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{tmpDir})
		if code == 0 {
			t.Errorf("Expected non-zero code for invalid file input")
		}
	})
}

// TestMainBadJSON ensures we fail on malformed JSON.
func TestMainBadJSON(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainBadJSON", flag.ContinueOnError)

	tmpFile, err := os.CreateTemp("", "bad_json_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{invalid json")
	tmpFile.Close()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{tmpFile.Name()})
		if code == 0 {
			t.Errorf("Expected non-zero exit code for malformed JSON")
		}
	})
}

// TestMainWriteFailure ensures we fail if the file is not writable.
func TestMainWriteFailure(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainWriteFailure", flag.ContinueOnError)

	tmpFile, err := os.CreateTemp("", "domains_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	domainData := []DomainRecord{{Domain: "example.com"}}
	inputJSON, _ := json.MarshalIndent(domainData, "", "  ")
	if _, err := tmpFile.Write(inputJSON); err != nil {
		t.Fatalf("Failed to write JSON: %v", err)
	}
	tmpFile.Close()

	// Make read-only
	if err := os.Chmod(tmpFile.Name(), 0400); err != nil {
		t.Fatalf("Failed chmod: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
				io.WriteString(c, "No match for example.com\n")
				time.Sleep(50 * time.Millisecond)
			}(conn)
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{"--whois=" + ln.Addr().String(), tmpFile.Name()})
		if code == 0 {
			t.Errorf("Expected non-zero exit code on file write failure")
		}
	})
}

// TestMainSleepVerifiesDelay checks the approximate delay when the --sleep flag is used.
func TestMainSleepVerifiesDelay(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainSleepVerifiesDelay", flag.ContinueOnError)

	// We'll still test for a ~2-second delay, but let's confirm that the user can override it.
	// Here we set it to 1s to speed up. We measure the total time and verify the sleeps happen.

	tmpFile, err := os.CreateTemp("", "domains_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	domainData := []DomainRecord{
		{Domain: "example1.com"},
		{Domain: "example2.com"},
	}
	inputJSON, _ := json.MarshalIndent(domainData, "", "  ")
	if _, err := tmpFile.Write(inputJSON); err != nil {
		t.Fatalf("Failed to write JSON: %v", err)
	}
	tmpFile.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
				io.WriteString(c, "No match for domain\n")
				time.Sleep(50 * time.Millisecond)
			}(conn)
		}
	}()

	start := time.Now()
	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			"--sleep=1s",
			tmpFile.Name(),
		})
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}
	})
	elapsed := time.Since(start)

	// We set sleep=1s. With 2 domains, that's ~2s total, plus overhead. We expect > 1.5s at least.
	if elapsed < 1500*time.Millisecond {
		t.Errorf("Expected total run time >= 1.5s, got %v", elapsed)
	}
}

// TestDomainRecordStructure ensures we haven't broken JSON compatibility.
func TestDomainRecordStructure(t *testing.T) {
	dr := DomainRecord{
		Domain:    "test.com",
		Log:       "some log",
		Available: true,
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Failed to marshal DomainRecord: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	expectedFields := []string{"domain", "log", "available"}
	for _, f := range expectedFields {
		if _, ok := out[f]; !ok {
			t.Errorf("Missing field %q in DomainRecord JSON", f)
		}
	}
}
