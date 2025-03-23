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
)

// captureOutput captures stdout/stderr during a function call.
func captureOutput(t *testing.T, fn func()) (string, string) {
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr

	outCh := make(chan string)
	errCh := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()

	fn()

	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	return <-outCh, <-errCh
}

// TestCheckDomainAvailability covers basic WHOIS availability checks
func TestCheckDomainAvailability(t *testing.T) {
	cases := []struct {
		name          string
		serverHandler func(net.Conn)
		wantAvailable bool
		wantReason    AvailabilityReason
		wantErr       bool
	}{
		{
			name: "No match => available=TRUE, reason=NO_MATCH",
			serverHandler: func(c net.Conn) {
				// Just respond with the magic string
				io.Copy(io.Discard, c)
				io.WriteString(c, "No match for example.com\n")
				c.Close()
			},
			wantAvailable: true,
			wantReason:    ReasonNoMatch,
		},
		{
			name: "Domain found => available=FALSE, reason=TAKEN",
			serverHandler: func(c net.Conn) {
				io.Copy(io.Discard, c)
				io.WriteString(c, "Domain Name: something.com\n")
				c.Close()
			},
			wantAvailable: false,
			wantReason:    ReasonTaken,
		},
		{
			name: "Immediate close => reason=ERROR",
			serverHandler: func(c net.Conn) {
				c.Close()
			},
			wantAvailable: false,
			wantReason:    ReasonError,
			wantErr:       true,
		},
		{
			name: "Empty response => reason=ERROR",
			serverHandler: func(c net.Conn) {
				// Send no data
				io.Copy(io.Discard, c)
				c.Close()
			},
			wantAvailable: false,
			wantReason:    ReasonError,
			wantErr:       true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("failed to listen: %v", err)
			}
			defer ln.Close()

			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				tt.serverHandler(conn)
			}()

			avail, reason, _, err := checkDomainAvailability("example.com", ln.Addr().String())
			if tt.wantErr && err == nil {
				t.Errorf("expected an error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("did not expect error, got: %v", err)
			}
			if avail != tt.wantAvailable {
				t.Errorf("got available=%v, want %v", avail, tt.wantAvailable)
			}
			if reason != tt.wantReason {
				t.Errorf("got reason=%q, want %q", reason, tt.wantReason)
			}
		})
	}
}

// TestArgParsing checks we fail if no arguments or if whois is missing
func TestArgParsing(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestArgParsing", flag.ContinueOnError)

	// No args
	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{})
		if code == 0 {
			t.Error("Expected non-zero code with no args")
		}
	})
	if !strings.Contains(stderr, "Usage: talia") {
		t.Errorf("Expected usage help, got: %s", stderr)
	}

	// Arg but no --whois
	flag.CommandLine = flag.NewFlagSet("TestArgParsingNoWhois", flag.ContinueOnError)
	_, stderr = captureOutput(t, func() {
		code := runCLI([]string{"somefile.json"})
		if code == 0 {
			t.Error("Expected non-zero code if whois is missing")
		}
	})
	if !strings.Contains(stderr, "Error: --whois=<server:port> is required") {
		t.Errorf("Expected missing whois error, got: %s", stderr)
	}
}

// TestInputFileReadError ensures we fail if input file can't be read
func TestInputFileReadError(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestInputFileReadError", flag.ContinueOnError)

	// We'll pass a directory instead of a file
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{"--whois=127.0.0.1:9999", tmpDir})
		if code == 0 {
			t.Errorf("Expected non-zero code for read error")
		}
	})
	if !strings.Contains(stderr, "Error reading") {
		t.Errorf("Expected 'Error reading' message, got: %s", stderr)
	}
}

// TestInputFileParseError ensures we fail on malformed JSON
func TestInputFileParseError(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestInputFileParseError", flag.ContinueOnError)

	tmpFile, err := os.CreateTemp("", "badjson_*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("{\"domain\": \"test.com\", ") // incomplete
	tmpFile.Close()

	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{"--whois=127.0.0.1:9999", tmpFile.Name()})
		if code == 0 {
			t.Errorf("Expected non-zero code for JSON parse error")
		}
	})
	if !strings.Contains(stderr, "Error parsing JSON") {
		t.Errorf("Expected parse error, got: %s", stderr)
	}
}

// TestMainNonGrouped verifies inline updates in non-grouped mode
func TestMainNonGrouped(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainNonGrouped", flag.ContinueOnError)

	tmp, err := os.CreateTemp("", "domains_*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	domains := []DomainRecord{
		{Domain: "example1.com"},
		{Domain: "example2.com"},
	}
	js, _ := json.MarshalIndent(domains, "", "  ")
	if _, err := tmp.Write(js); err != nil {
		t.Fatalf("Failed writing to temp file: %v", err)
	}
	tmp.Close()

	// WHOIS => always 'No match for' => available
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			io.Copy(io.Discard, c)
			io.WriteString(c, "No match for domain\n")
			c.Close()
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			tmp.Name(),
		})
		if code != 0 {
			t.Errorf("want exit code 0, got %d", code)
		}
	})

	updated, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("reading updated file: %v", err)
	}
	var updatedList []DomainRecord
	if err := json.Unmarshal(updated, &updatedList); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	if len(updatedList) != 2 {
		t.Errorf("want 2, got %d", len(updatedList))
	}
	for _, r := range updatedList {
		if !r.Available {
			t.Errorf("domain %s expected available=true", r.Domain)
		}
		if r.Reason != ReasonNoMatch {
			t.Errorf("domain %s reason: got %s want NO_MATCH", r.Domain, r.Reason)
		}
	}
}

// TestFileWriteError ensures we fail if we can't write the input file in non-grouped mode
func TestFileWriteError(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestFileWriteError", flag.ContinueOnError)

	tmp, err := os.CreateTemp("", "domains_write_err_*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	// put some data
	domains := []DomainRecord{{Domain: "test.com"}}
	b, _ := json.Marshal(domains)
	tmp.Write(b)
	tmp.Close()

	// Make it read-only
	os.Chmod(tmp.Name(), 0400)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			io.Copy(io.Discard, c)
			io.WriteString(c, "No match for domain\n")
			c.Close()
		}
	}()

	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			tmp.Name(),
		})
		if code == 0 {
			t.Error("Expected non-zero code on file write error")
		}
	})
	if !strings.Contains(stderr, "Error writing file") {
		t.Errorf("Expected file write error, got: %s", stderr)
	}
}

// TestMainVerbose checks if verbose logs are stored for successful checks
func TestMainVerbose(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainVerbose", flag.ContinueOnError)

	tmp, err := os.CreateTemp("", "domains_verbose_*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	domains := []DomainRecord{{Domain: "verbose1.com"}}
	js, _ := json.MarshalIndent(domains, "", "  ")
	tmp.Write(js)
	tmp.Close()

	// WHOIS => "No match for"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			io.Copy(io.Discard, c)
			io.WriteString(c, "No match for domain\n")
			c.Close()
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--verbose",
			"--whois=" + ln.Addr().String(),
			tmp.Name(),
		})
		if code != 0 {
			t.Errorf("want exit code 0, got %d", code)
		}
	})

	updated, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}
	var updatedList []DomainRecord
	json.Unmarshal(updated, &updatedList)
	if len(updatedList) != 1 {
		t.Errorf("want 1 domain, got %d", len(updatedList))
	}
	if updatedList[0].Log == "" {
		t.Error("expected a WHOIS log in verbose mode, got empty string")
	}
}

// TestMainErrorCase verifies that if one domain triggers an error, we still proceed
func TestMainErrorCase(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainErrorCase", flag.ContinueOnError)

	tmp, err := os.CreateTemp("", "domains_error_*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	domains := []DomainRecord{
		{Domain: "error1.com"},
		{Domain: "ok2.com"},
	}
	js, _ := json.MarshalIndent(domains, "", "  ")
	tmp.Write(js)
	tmp.Close()

	// First conn => immediate close => error, second => "No match for"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		counter := 0
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			func(conn net.Conn) {
				defer conn.Close()
				io.Copy(io.Discard, conn)
				if counter == 0 {
					// Immediate close => error
				} else {
					io.WriteString(conn, "No match for domain\n")
				}
				counter++
			}(c)
		}
	}()

	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			tmp.Name(),
		})
		if code != 0 {
			t.Errorf("want 0, got %d", code)
		}
	})

	if !strings.Contains(stderr, "WHOIS error for error1.com") {
		t.Errorf("Expected error message for domain error1.com, got: %s", stderr)
	}

	updated, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}
	var updatedList []DomainRecord
	if err := json.Unmarshal(updated, &updatedList); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(updatedList) != 2 {
		t.Errorf("want 2, got %d", len(updatedList))
	}
	// error1.com => reason=ERROR
	// ok2.com => reason=NO_MATCH
	if updatedList[0].Domain == "error1.com" {
		if updatedList[0].Reason != ReasonError {
			t.Errorf("expected reason=ERROR for error1.com, got %s", updatedList[0].Reason)
		}
	} else {
		t.Errorf("Unexpected domain ordering for the first record")
	}
	if updatedList[1].Domain == "ok2.com" {
		if updatedList[1].Reason != ReasonNoMatch {
			t.Errorf("expected reason=NO_MATCH for ok2.com, got %s", updatedList[1].Reason)
		}
	} else {
		t.Errorf("Unexpected domain ordering for the second record")
	}
}

// TestMainGroupedNoFile overwrites input JSON with a grouped object
func TestMainGroupedNoFile(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedNoFile", flag.ContinueOnError)

	tmp, err := os.CreateTemp("", "grouped_no_file_*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	domains := []DomainRecord{
		{Domain: "g1.com"},
		{Domain: "g2.com"},
	}
	js, _ := json.MarshalIndent(domains, "", "  ")
	tmp.Write(js)
	tmp.Close()

	// WHOIS => first => no match => available, second => domain => taken
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		i := 0
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			func(conn net.Conn) {
				defer conn.Close()
				io.Copy(io.Discard, conn)
				if i == 0 {
					io.WriteString(conn, "No match for domain\n")
				} else {
					io.WriteString(conn, "Domain Name: something.com\n")
				}
				i++
			}(c)
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--grouped-output",
			"--whois=" + ln.Addr().String(),
			tmp.Name(),
		})
		if code != 0 {
			t.Errorf("wanted exit code 0, got %d", code)
		}
	})

	groupedBytes, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("reading grouped: %v", err)
	}
	var grouped struct {
		Available   []map[string]string `json:"available"`
		Unavailable []map[string]string `json:"unavailable"`
	}
	if err := json.Unmarshal(groupedBytes, &grouped); err != nil {
		t.Fatalf("unmarshal grouped: %v", err)
	}
	if len(grouped.Available) != 1 || len(grouped.Unavailable) != 1 {
		t.Errorf("expected 1 avail + 1 unavail, got %d/%d", len(grouped.Available), len(grouped.Unavailable))
	}
}

// TestMainGroupedWithFile merges results into a separate grouped file
func TestMainGroupedWithFile(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedWithFile", flag.ContinueOnError)

	// Input
	inputFile, err := os.CreateTemp("", "grouped_input_*.json")
	if err != nil {
		t.Fatalf("temp input: %v", err)
	}
	defer os.Remove(inputFile.Name())

	domains := []DomainRecord{{Domain: "merge-test1.com"}, {Domain: "merge-test2.com"}}
	data, _ := json.MarshalIndent(domains, "", "  ")
	inputFile.Write(data)
	inputFile.Close()

	// Existing grouped file (non-empty) with 1 domain
	groupedFile, err := os.CreateTemp("", "grouped_output_*.json")
	if err != nil {
		t.Fatalf("temp grouped: %v", err)
	}
	defer os.Remove(groupedFile.Name())

	existingGrouped := GroupedData{
		Available: []GroupedDomain{
			{Domain: "previous-available.com", Reason: ReasonNoMatch},
		},
		Unavailable: []GroupedDomain{
			{Domain: "previous-unavailable.com", Reason: ReasonTaken},
		},
	}
	egBytes, _ := json.MarshalIndent(existingGrouped, "", "  ")
	groupedFile.Write(egBytes)
	groupedFile.Close()

	// WHOIS => first => no match => available, second => found => unavailable
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		i := 0
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
				if i == 0 {
					io.WriteString(c, "No match for domain\n")
				} else {
					io.WriteString(c, "Domain Name: found-something\n")
				}
				i++
			}(conn)
		}
	}()

	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--grouped-output",
			"--output-file=" + groupedFile.Name(),
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if code != 0 {
			t.Errorf("wanted 0, got %d", code)
		}
	})

	// Input file should be unchanged
	inRaw, err := os.ReadFile(inputFile.Name())
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	var inCheck []DomainRecord
	json.Unmarshal(inRaw, &inCheck)
	for _, r := range inCheck {
		if r.Available || r.Reason != "" {
			t.Errorf("expected no changes in input JSON, found reason=%s", r.Reason)
		}
	}

	// Grouped file => old plus newly inserted
	gRaw, err := os.ReadFile(groupedFile.Name())
	if err != nil {
		t.Fatalf("read groupedFile: %v", err)
	}
	var merged GroupedData
	if err := json.Unmarshal(gRaw, &merged); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}
	// We had 1 in Available, 1 in Unavailable,
	// plus 1 new in Available, 1 new in Unavailable.
	if len(merged.Available) != 2 {
		t.Errorf("expected 2 available, got %d", len(merged.Available))
	}
	if len(merged.Unavailable) != 2 {
		t.Errorf("expected 2 unavailable, got %d", len(merged.Unavailable))
	}
}

// TestMainGroupedFileEmptyExisting ensures no crash if grouped file is empty
func TestMainGroupedFileEmptyExisting(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedFileEmptyExisting", flag.ContinueOnError)

	// Input
	inputFile, err := os.CreateTemp("", "grouped_input_*.json")
	if err != nil {
		t.Fatalf("temp input: %v", err)
	}
	defer os.Remove(inputFile.Name())

	domains := []DomainRecord{{Domain: "empty-existing-1.com"}}
	data, _ := json.MarshalIndent(domains, "", "  ")
	inputFile.Write(data)
	inputFile.Close()

	// Create an empty grouped file
	groupedFile, err := os.CreateTemp("", "grouped_out_*.json")
	if err != nil {
		t.Fatalf("temp grouped: %v", err)
	}
	groupedFile.Close()
	defer os.Remove(groupedFile.Name())

	// WHOIS => no match => available
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		io.Copy(io.Discard, c)
		io.WriteString(c, "No match for domain\n")
		c.Close()
	}()

	_, stderr := captureOutput(t, func() {
		code := runCLI([]string{
			"--grouped-output",
			"--output-file=" + groupedFile.Name(),
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}
	})

	if strings.Contains(stderr, "unexpected end of JSON") {
		t.Errorf("Should not fail on empty grouped file: %s", stderr)
	}

	// Confirm the grouped file now has a single available domain
	gRaw, err := os.ReadFile(groupedFile.Name())
	if err != nil {
		t.Fatalf("read grouped file: %v", err)
	}
	var merged GroupedData
	if err := json.Unmarshal(gRaw, &merged); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}
	if len(merged.Available) != 1 {
		t.Errorf("expected 1 available, got %d", len(merged.Available))
	}
	if len(merged.Unavailable) != 0 {
		t.Errorf("expected 0 unavailable, got %d", len(merged.Unavailable))
	}
}

// TestDomainRecordStructure ensures JSON keys remain stable
func TestDomainRecordStructure(t *testing.T) {
	dr := DomainRecord{
		Domain:    "example.com",
		Available: true,
		Reason:    ReasonNoMatch,
		Log:       "some log data",
	}
	b, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fields := []string{"domain", "available", "reason", "log"}
	for _, f := range fields {
		if _, ok := parsed[f]; !ok {
			t.Errorf("missing field %q in DomainRecord JSON", f)
		}
	}
}
