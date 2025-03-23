// Package main provides the Talia CLI tool, which checks domain availability
// via WHOIS and updates JSON files or produces grouped results.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// AvailabilityReason is a short code explaining domain availability
type AvailabilityReason string

const (
	ReasonNoMatch AvailabilityReason = "NO_MATCH"
	ReasonTaken   AvailabilityReason = "TAKEN"
	ReasonError   AvailabilityReason = "ERROR"
)

// DomainRecord is how we parse the input array in non-grouped mode.
// "available" and "reason" are overwritten by Talia in non-grouped mode.
type DomainRecord struct {
	Domain    string             `json:"domain"`
	Available bool               `json:"available"`
	Reason    AvailabilityReason `json:"reason,omitempty"`
	Log       string             `json:"log,omitempty"`
}

// GroupedDomain is a minimal record for grouped output: domain & reason only.
type GroupedDomain struct {
	Domain string             `json:"domain"`
	Reason AvailabilityReason `json:"reason"`
}

// GroupedData is the top-level object for grouped JSON. It has two arrays:
// "available" and "unavailable", each containing objects with domain + reason.
type GroupedData struct {
	Available   []GroupedDomain `json:"available"`
	Unavailable []GroupedDomain `json:"unavailable"`
}

// checkDomainAvailability queries the WHOIS server for a single domain.
func checkDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return false, ReasonError, "", fmt.Errorf("failed to connect to WHOIS: %w", err)
	}
	defer conn.Close()

	// Send the domain
	_, _ = fmt.Fprintf(conn, "%s\r\n", domain)

	// If it's a TCPConn, close write side
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}

	data, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		return false, ReasonError, "", fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return false, ReasonError, "", fmt.Errorf("empty WHOIS response")
	}
	response := string(data)

	if strings.Contains(response, "No match for") {
		return true, ReasonNoMatch, response, nil
	}
	return false, ReasonTaken, response, nil
}

// mergeGrouped merges new grouped results into existing grouped data, deduplicating by domain.
func mergeGrouped(existing, newest GroupedData) GroupedData {
	domainsAvail := make(map[string]AvailabilityReason)
	for _, gd := range existing.Available {
		domainsAvail[gd.Domain] = gd.Reason
	}
	domainsUnavail := make(map[string]AvailabilityReason)
	for _, gd := range existing.Unavailable {
		domainsUnavail[gd.Domain] = gd.Reason
	}

	// Insert or update from newest
	for _, gd := range newest.Available {
		domainsAvail[gd.Domain] = gd.Reason
		// If it was in unavail, remove it
		delete(domainsUnavail, gd.Domain)
	}
	for _, gd := range newest.Unavailable {
		domainsUnavail[gd.Domain] = gd.Reason
		// If it was in avail, remove it
		delete(domainsAvail, gd.Domain)
	}

	// Rebuild arrays
	out := GroupedData{}
	for d, r := range domainsAvail {
		out.Available = append(out.Available, GroupedDomain{Domain: d, Reason: r})
	}
	for d, r := range domainsUnavail {
		out.Unavailable = append(out.Unavailable, GroupedDomain{Domain: d, Reason: r})
	}
	return out
}

// writeGroupedFile reads an existing grouped JSON (if any), merges new data, and writes back.
func writeGroupedFile(path string, newest GroupedData) error {
	if path == "" {
		return nil
	}

	// Read existing if it exists
	existing := GroupedData{}
	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		// File exists, non-empty
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read grouped file: %w", err)
		}
		if err := json.Unmarshal(raw, &existing); err != nil {
			return fmt.Errorf("parse grouped file: %w", err)
		}
	}

	// Merge
	merged := mergeGrouped(existing, newest)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal grouped data: %w", err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write grouped file: %w", err)
	}
	return nil
}

// replaceDomain is used in non-grouped mode to update a domain's record in the slice.
func replaceDomain(domains []DomainRecord, rec DomainRecord) {
	for i, d := range domains {
		if d.Domain == rec.Domain {
			domains[i] = rec
			return
		}
	}
}

// runCLI is the main entry point for Talia logic.
func runCLI(args []string) int {
	fs := flag.NewFlagSet("talia", flag.ContinueOnError)
	whoisServer := fs.String("whois", "", "WHOIS server, e.g. whois.verisign-grs.com:43 (required)")
	sleep := fs.Duration("sleep", 2*time.Second, "Time to sleep between domain checks (default 2s)")
	verbose := fs.Bool("verbose", false, "Include WHOIS log in 'log' field even for successful checks")
	groupedOutput := fs.Bool("grouped-output", false, "Enable grouped output (JSON object with 'available','unavailable')")
	outputFile := fs.String("output-file", "", "Path to grouped output file (if set, input file remains unmodified)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s --whois=<server:port> [--sleep=2s] [--verbose] [--grouped-output] [--output-file=path] <json-file>\n", fs.Name())
		return 1
	}
	if *whoisServer == "" {
		fmt.Fprintln(os.Stderr, "Error: --whois=<server:port> is required")
		return 1
	}

	inputPath := fs.Arg(0)
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		return 1
	}

	// We'll parse the input as a slice of DomainRecord for non-grouped updates,
	// but we only use it fully if we're in non-grouped mode or grouped-without-output-file.
	var domains []DomainRecord
	if err := json.Unmarshal(raw, &domains); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON in %s: %v\n", inputPath, err)
		return 1
	}

	// We accumulate grouped results if needed
	groupedData := GroupedData{}

	for _, rec := range domains {
		fmt.Printf("Checking %s on %s\n", rec.Domain, *whoisServer)
		avail, reason, logData, err := checkDomainAvailability(rec.Domain, *whoisServer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WHOIS error for %s: %v\n", rec.Domain, err)
			avail = false
			reason = ReasonError
			logData = fmt.Sprintf("Error: %v", err)
		}

		if !*groupedOutput {
			// =========== Non-Grouped Mode ===========
			rec.Available = avail
			rec.Reason = reason
			if *verbose || reason == ReasonError {
				rec.Log = logData
			} else {
				rec.Log = ""
			}
			replaceDomain(domains, rec)

			// Write the updated array back to the same file after each domain
			out, err := json.MarshalIndent(domains, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				return 1
			}
			if err := os.WriteFile(inputPath, out, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
				return 1
			}

		} else {
			// =========== Grouped Mode ===========
			// We gather minimal domain+reason in the appropriate bucket
			gd := GroupedDomain{Domain: rec.Domain, Reason: reason}
			if avail {
				groupedData.Available = append(groupedData.Available, gd)
			} else {
				groupedData.Unavailable = append(groupedData.Unavailable, gd)
			}
		}

		time.Sleep(*sleep)
	}

	// Now handle final grouped output if we used --grouped-output
	if *groupedOutput {
		if *outputFile == "" {
			// Overwrite input file with { "available": [...], "unavailable": [...] }
			mergedOut, err := json.MarshalIndent(groupedData, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling grouped JSON: %v\n", err)
				return 1
			}
			if err := os.WriteFile(inputPath, mergedOut, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing grouped JSON to %s: %v\n", inputPath, err)
				return 1
			}
			fmt.Println("Processing complete in grouped-output mode (overwrote input).")
		} else {
			// Write or merge to the specified output file
			if err := writeGroupedFile(*outputFile, groupedData); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing grouped file: %v\n", err)
				return 1
			}
			fmt.Println("Processing complete in grouped-output mode (wrote to separate file).")
		}

	} else {
		// Non-grouped mode
		fmt.Println("Processing complete. Updated file:", inputPath)
	}

	return 0
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}
