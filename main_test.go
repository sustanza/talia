// Package main_test contains tests for Talia, a CLI for WHOIS-based domain checks.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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

// TestMainGroupedFileRepeatedAppend tests that we can append multiple times to the same grouped file
func TestMainGroupedFileRepeatedAppend(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedFileRepeatedAppend", flag.ContinueOnError)

	// 1. Create input: a plain domain list
	inputFile, err := os.CreateTemp("", "repeated_input_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(inputFile.Name())

	doms := []DomainRecord{
		{Domain: "append-1.com"},
		{Domain: "append-2.com"},
	}
	b, _ := json.Marshal(doms)
	inputFile.Write(b)
	inputFile.Close()

	// 2. Create/initialize an output grouped file
	groupedFile, err := os.CreateTemp("", "repeated_output_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(groupedFile.Name())

	gf := GroupedData{
		Available: []GroupedDomain{},
		Unavailable: []GroupedDomain{
			{Domain: "some-old-unavailable.com", Reason: ReasonTaken},
		},
	}
	gfBytes, _ := json.MarshalIndent(gf, "", "  ")
	groupedFile.Write(gfBytes)
	groupedFile.Close()

	// 3. Start a mock WHOIS server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		// We'll alternate: first => no match => available
		// second => domain found => taken
		i := 0
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			io.Copy(io.Discard, c)
			if i%2 == 0 {
				io.WriteString(c, "No match for domain\n")
			} else {
				io.WriteString(c, "Domain Name: found-it\n")
			}
			c.Close()
			i++
		}
	}()

	// 4. Run Talia with --grouped-output and the same groupedFile a first time
	_, _ = captureOutput(t, func() {
		exitCodeFirst := runCLI([]string{
			"--grouped-output",
			"--output-file=" + groupedFile.Name(),
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if exitCodeFirst != 0 {
			t.Fatalf("First runCLI failed with code=%d", exitCodeFirst)
		}
	})

	// 5. Re-run with the same EXACT input
	_, _ = captureOutput(t, func() {
		exitCodeSecond := runCLI([]string{
			"--grouped-output",
			"--output-file=" + groupedFile.Name(),
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if exitCodeSecond != 0 {
			t.Fatalf("Second runCLI failed with code=%d", exitCodeSecond)
		}
	})

	// 6. Inspect the grouped file
	mergedBytes, _ := os.ReadFile(groupedFile.Name())
	var final GroupedData
	if err := json.Unmarshal(mergedBytes, &final); err != nil {
		t.Fatal(err)
	}

	if len(final.Available) < 1 || len(final.Unavailable) < 2 {
		t.Errorf("Expected domains in available/unavailable after repeated merges, got %d/%d",
			len(final.Available), len(final.Unavailable))
	}

	// Check if the original unavailable domain is still there
	var foundOriginal bool
	for _, d := range final.Unavailable {
		if d.Domain == "some-old-unavailable.com" {
			foundOriginal = true
			break
		}
	}
	if !foundOriginal {
		t.Error("Original unavailable domain was lost during merges")
	}
}

// TestMainGroupedFileWithUnverifiedInput tests using a grouped file with unverified domains as input
func TestMainGroupedFileWithUnverifiedInput(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedFileWithUnverifiedInput", flag.ContinueOnError)

	// We'll build a grouped JSON with a few unverified domains
	g := ExtendedGroupedData{
		Available: []GroupedDomain{
			{Domain: "old-avail.com", Reason: ReasonNoMatch},
		},
		Unavailable: []GroupedDomain{
			{Domain: "old-unavail.com", Reason: ReasonTaken},
		},
		Unverified: []DomainRecord{
			{Domain: "check-me-1.com"},
			{Domain: "check-me-2.com"},
		},
	}
	raw, _ := json.MarshalIndent(g, "", "  ")

	// Write it to a temp file
	inputFile, err := os.CreateTemp("", "grouped_unverified_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(inputFile.Name())
	inputFile.Write(raw)
	inputFile.Close()

	// Start WHOIS server that returns "No match for" for one domain
	// and "Domain Name:" for the other.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		i := 0
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			io.Copy(io.Discard, c)
			if i%2 == 0 {
				io.WriteString(c, "No match for domain\n")
			} else {
				io.WriteString(c, "Domain Name: found-something\n")
			}
			c.Close()
			i++
		}
	}()

	_, _ = captureOutput(t, func() {
		exitCode := runCLI([]string{
			"--grouped-output",
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if exitCode != 0 {
			t.Fatalf("runCLI failed with code=%d", exitCode)
		}
	})

	// Now read back the file, it should have no unverified, and the 2 domains
	// should be moved into available/unavailable.
	updatedBytes, _ := os.ReadFile(inputFile.Name())
	var updated ExtendedGroupedData
	if err := json.Unmarshal(updatedBytes, &updated); err != nil {
		t.Fatal(err)
	}

	if len(updated.Unverified) != 0 {
		t.Errorf("Expected unverified to be empty after checking, got %d", len(updated.Unverified))
	}

	// We had 1 domain "No match for" => available
	// plus 1 domain "Domain Name:" => unavailable
	// plus the old ones
	if len(updated.Available) != 2 {
		t.Errorf("Expected 2 in available, got %d", len(updated.Available))
	}
	if len(updated.Unavailable) != 2 {
		t.Errorf("Expected 2 in unavailable, got %d", len(updated.Unavailable))
	}

	// Check if the original domains are still there
	var foundOldAvail, foundOldUnavail bool
	for _, d := range updated.Available {
		if d.Domain == "old-avail.com" {
			foundOldAvail = true
			break
		}
	}
	for _, d := range updated.Unavailable {
		if d.Domain == "old-unavail.com" {
			foundOldUnavail = true
			break
		}
	}
	if !foundOldAvail || !foundOldUnavail {
		t.Error("Original domains were lost during processing")
	}
}

// TestCheckDomainAvailability_DialError verifies we get an error/log when net.Dial fails.
func TestCheckDomainAvailability_DialError(t *testing.T) {
	// Use a port that is presumably not open.
	addr := "127.0.0.1:1"

	available, reason, logData, err := checkDomainAvailability("faildial.com", addr)
	if err == nil {
		t.Errorf("Expected error from net.Dial, got nil")
	}
	if reason != ReasonError {
		t.Errorf("Expected ReasonError for dial failure, got %s", reason)
	}
	if available {
		t.Error("Expected domain NOT to be available on dial failure")
	}
	if logData == "" {
		t.Error("Expected some error log data, got empty string")
	}
}

// TestCheckDomainAvailability_ReadError tries to trigger a partial read error
func TestCheckDomainAvailability_ReadError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	// Server that writes partial data, then forcibly resets the connection
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Write([]byte("Partial WHOIS data..."))
			if tcp, ok := conn.(*net.TCPConn); ok {
				// Attempt to force an RST packet
				tcp.SetLinger(0)
			}
			conn.Close()
		}
	}()

	_, reason, _, err2 := checkDomainAvailability("partialread.com", ln.Addr().String())
	if err2 == nil {
		t.Error("Expected a non-EOF error from partial read, got nil")
	}
	if reason != ReasonError {
		t.Errorf("Expected reason=ERROR for partial read, got %s", reason)
	}
}

// TestReplaceDomain_NotFound ensures we cover the scenario where replaceDomain
// doesn't find a matching domain.
func TestReplaceDomain_NotFound(t *testing.T) {
	original := []DomainRecord{
		{Domain: "existing.com", Available: false},
	}
	newRec := DomainRecord{Domain: "not-found.com", Available: true}

	replaceDomain(original, newRec)
	// The array should remain unchanged if domain not found
	if len(original) != 1 || original[0].Domain != "existing.com" {
		t.Error("replaceDomain incorrectly replaced a non-existent domain")
	}
}

func TestWriteGroupedFile_EmptyPath(t *testing.T) {
	err := writeGroupedFile("", GroupedData{})
	if err != nil {
		t.Errorf("Expected nil error if path==\"\", got %v", err)
	}
}

func TestWriteGroupedFile_NewFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "dummy_*")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(tmpPath)

	gData := GroupedData{
		Available: []GroupedDomain{{Domain: "newavail.com", Reason: ReasonNoMatch}},
	}

	err = writeGroupedFile(tmpPath, gData)
	if err != nil {
		t.Fatalf("writeGroupedFile returned error: %v", err)
	}

	raw, _ := os.ReadFile(tmpPath)
	defer os.Remove(tmpPath)

	var out GroupedData
	json.Unmarshal(raw, &out)
	if len(out.Available) != 1 || out.Available[0].Domain != "newavail.com" {
		t.Errorf("Expected domain newavail.com in available, got %+v", out)
	}
}

func TestWriteGroupedFile_ParseArrayFallback(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "array_fallback_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	arr := []DomainRecord{
		{Domain: "arrdomain1.com", Available: true, Reason: ReasonNoMatch},
		{Domain: "arrdomain2.com", Available: false, Reason: ReasonTaken},
	}
	raw, _ := json.Marshal(arr)
	tmpFile.Write(raw)
	tmpFile.Close()

	newest := GroupedData{
		Available: []GroupedDomain{
			{Domain: "additionalAvail.com", Reason: ReasonNoMatch},
		},
	}

	err = writeGroupedFile(tmpFile.Name(), newest)
	if err != nil {
		t.Fatalf("writeGroupedFile error: %v", err)
	}

	finalRaw, _ := os.ReadFile(tmpFile.Name())
	var final GroupedData
	json.Unmarshal(finalRaw, &final)

	if len(final.Available) != 2 {
		t.Errorf("Expected 2 available, got %d: %#v", len(final.Available), final.Available)
	}
	if len(final.Unavailable) != 1 {
		t.Errorf("Expected 1 unavailable, got %d: %#v", len(final.Unavailable), final.Unavailable)
	}
}

type UnmarshalableGroupedData struct {
	GroupedData
	BadField func() `json:"bad_field"` // This breaks json.Marshal
}

func TestWriteGroupedFile_MarshalError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "marshal_err_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	g := UnmarshalableGroupedData{
		GroupedData: GroupedData{
			Available: []GroupedDomain{{Domain: "bad-marsh.com", Reason: ReasonNoMatch}},
		},
		BadField: func() {},
	}

	err = testWriteGroupedFileWithInterface(tmpFile.Name(), g)
	if err == nil || !strings.Contains(err.Error(), "marshal grouped data") {
		t.Errorf("Expected marshal error, got %v", err)
	}
}

// Minimal clone that forcibly calls json.MarshalIndent on 'data'
func testWriteGroupedFileWithInterface(path string, data interface{}) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal grouped data: %w", err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write grouped file: %w", err)
	}
	return nil
}

// NEW TEST #1: Directly test convertArrayToGrouped
func TestConvertArrayToGrouped(t *testing.T) {
	arr := []DomainRecord{
		{Domain: "test1.com", Available: true, Reason: ReasonNoMatch, Log: "log1"},
		{Domain: "test2.com", Available: false, Reason: ReasonTaken, Log: "log2"},
		{Domain: "test3.com", Available: false, Reason: ReasonError, Log: "log3"},
	}
	g := convertArrayToGrouped(arr)
	if len(g.Available) != 1 {
		t.Errorf("expected 1 in available, got %d", len(g.Available))
	}
	if len(g.Unavailable) != 2 {
		t.Errorf("expected 2 in unavailable, got %d", len(g.Unavailable))
	}

	if g.Available[0].Domain != "test1.com" || g.Available[0].Log != "log1" {
		t.Error("test1.com should be in available with correct log")
	}

	var foundTest2, foundTest3 bool
	for _, u := range g.Unavailable {
		if u.Domain == "test2.com" && u.Reason == ReasonTaken && u.Log == "log2" {
			foundTest2 = true
		}
		if u.Domain == "test3.com" && u.Reason == ReasonError && u.Log == "log3" {
			foundTest3 = true
		}
	}
	if !foundTest2 || !foundTest3 {
		t.Error("Did not find test2.com or test3.com in unavailable list with correct reason/log")
	}
}

// NEW TEST #2: Directly test writeGroupedFile when existing file is valid grouped JSON
func TestWriteGroupedFile_ExistingGrouped(t *testing.T) {
	tmp, err := os.CreateTemp("", "existing_grouped_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	existing := GroupedData{
		Available:   []GroupedDomain{{Domain: "oldavail.com", Reason: ReasonNoMatch}},
		Unavailable: []GroupedDomain{{Domain: "oldunavail.com", Reason: ReasonTaken}},
	}
	oldBytes, _ := json.Marshal(existing)
	tmp.Write(oldBytes)
	tmp.Close()

	newData := GroupedData{
		Available: []GroupedDomain{
			{Domain: "newavail.com", Reason: ReasonNoMatch},
		},
		Unavailable: []GroupedDomain{
			{Domain: "newunavail.com", Reason: ReasonError},
		},
	}
	if err := writeGroupedFile(tmp.Name(), newData); err != nil {
		t.Fatalf("writeGroupedFile: %v", err)
	}

	finalRaw, _ := os.ReadFile(tmp.Name())
	var final GroupedData
	if err := json.Unmarshal(finalRaw, &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	// Expect old + new in each group
	if len(final.Available) != 2 {
		t.Errorf("expected 2 in available, got %d", len(final.Available))
	}
	if len(final.Unavailable) != 2 {
		t.Errorf("expected 2 in unavailable, got %d", len(final.Unavailable))
	}

	var haveOldAvail, haveNewAvail, haveOldUnavail, haveNewUnavail bool
	for _, d := range final.Available {
		if d.Domain == "oldavail.com" {
			haveOldAvail = true
		}
		if d.Domain == "newavail.com" {
			haveNewAvail = true
		}
	}
	for _, d := range final.Unavailable {
		if d.Domain == "oldunavail.com" {
			haveOldUnavail = true
		}
		if d.Domain == "newunavail.com" {
			haveNewUnavail = true
		}
	}
	if !haveOldAvail || !haveNewAvail || !haveOldUnavail || !haveNewUnavail {
		t.Errorf("Merge logic failed. Final: %+v", final)
	}
}

// NEW TEST #3: Test runCLIGroupedInput scenario WITHOUT --grouped-output (forces overwrite of the same file)
func TestRunCLIGroupedInput_Overwrite(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestRunCLIGroupedInput_Overwrite", flag.ContinueOnError)

	// Build a grouped JSON with unverified domain
	g := ExtendedGroupedData{
		Available: []GroupedDomain{
			{Domain: "old-avail.com", Reason: ReasonNoMatch},
		},
		Unavailable: []GroupedDomain{
			{Domain: "old-unavail.com", Reason: ReasonTaken},
		},
		Unverified: []DomainRecord{
			{Domain: "check-me-3.com"},
		},
	}
	b, _ := json.MarshalIndent(g, "", "  ")

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "grouped_overwrite_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(b)
	tmpFile.Close()

	// Start WHOIS => "Domain Name: found-something" => taken
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		if c != nil {
			io.Copy(io.Discard, c)
			io.WriteString(c, "Domain Name: found-something\n")
			c.Close()
		}
	}()

	// call runCLI with NO --grouped-output => runCLI sees grouped JSON, calls runCLIGroupedInput,
	// which sets finalOutputFile = inputPath if !groupedOutput.
	_, _ = captureOutput(t, func() {
		code := runCLI([]string{
			"--whois=" + ln.Addr().String(),
			tmpFile.Name(),
		})
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}
	})

	// Now the original file should be overwritten. unverified => empty, new domain => "taken"
	updated, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	var final ExtendedGroupedData
	if err := json.Unmarshal(updated, &final); err != nil {
		t.Fatalf("Unmarshal final: %v", err)
	}
	if len(final.Unverified) != 0 {
		t.Errorf("Expected no unverified, got %d", len(final.Unverified))
	}
	foundCheckMe3 := false
	for _, d := range final.Unavailable {
		if d.Domain == "check-me-3.com" {
			foundCheckMe3 = true
			if d.Reason != ReasonTaken {
				t.Errorf("Expected reason=TAKEN, got %s", d.Reason)
			}
		}
	}
	if !foundCheckMe3 {
		t.Errorf("Did not find 'check-me-3.com' in final unavailable: %+v", final.Unavailable)
	}
}

func TestMainGroupedFileWithUnverifiedInput_SeparateOutput(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("TestMainGroupedFileWithUnverifiedInput_SeparateOutput", flag.ContinueOnError)

	// Input: ExtendedGroupedData with unverified
	ext := ExtendedGroupedData{
		Available: []GroupedDomain{
			{Domain: "old-avail.com", Reason: ReasonNoMatch},
		},
		Unavailable: []GroupedDomain{
			{Domain: "old-unavail.com", Reason: ReasonTaken},
		},
		Unverified: []DomainRecord{
			{Domain: "need-check.com"},
		},
	}
	inputFile, _ := os.CreateTemp("", "unverified_separate_input_*.json")
	defer os.Remove(inputFile.Name())
	json.NewEncoder(inputFile).Encode(ext)
	inputFile.Close()

	// Output file (distinct from input)
	outFile, _ := os.CreateTemp("", "unverified_separate_out_*.json")
	outFileName := outFile.Name()
	outFile.Close()
	defer os.Remove(outFileName)

	// WHOIS => domain found => reason=TAKEN
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		io.Copy(io.Discard, c)
		io.WriteString(c, "Domain Name: found-something\n")
		c.Close()
	}()

	// Run with grouped-output + separate --output-file
	stdout, stderr := captureOutput(t, func() {
		code := runCLI([]string{
			"--grouped-output",
			"--output-file=" + outFileName,
			"--whois=" + ln.Addr().String(),
			inputFile.Name(),
		})
		if code != 0 {
			t.Errorf("Expected exit=0, got %d", code)
		}
	})

	// Check that we did NOT see "overwrote original file" message
	// Instead we expect "wrote results to: outFileName"
	if strings.Contains(stdout, "overwrote original file") {
		t.Error("Expected separate-file message, got overwrite message!")
	}
	if !strings.Contains(stdout, "wrote results to:") {
		t.Errorf("Missing the 'wrote results to:' line. stdout=%s\nstderr=%s", stdout, stderr)
	}

	// The outFile should now contain unverified => gone, domain => "need-check.com" in .Unavailable
	data, _ := os.ReadFile(outFileName)
	var final ExtendedGroupedData
	json.Unmarshal(data, &final)
	if len(final.Unverified) != 0 {
		t.Errorf("Expected unverified=0, got %d", len(final.Unverified))
	}
	if len(final.Unavailable) != 2 { // old + new
		t.Errorf("Expected 2 in .Unavailable, got %d", len(final.Unavailable))
	}
}

func TestWriteGroupedFile_CorruptExisting(t *testing.T) {
	tmp, err := os.CreateTemp("", "corrupt_grouped_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	// Write invalid JSON that fails parse as GroupedData & DomainRecord array
	tmp.WriteString(`{"not_valid": `) // truncated, invalid
	tmp.Close()

	newest := GroupedData{
		Available: []GroupedDomain{{Domain: "wontexist.com", Reason: ReasonNoMatch}},
	}

	// Should fail with parse grouped file
	err = writeGroupedFile(tmp.Name(), newest)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse grouped file") {
		t.Errorf("Expected parse grouped file error, got %v", err)
	}
}
