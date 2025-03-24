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

// GroupedDomain is a minimal record for grouped output.
// We now include a Log field as well, so logs can be preserved in grouped mode.
type GroupedDomain struct {
	Domain string             `json:"domain"`
	Reason AvailabilityReason `json:"reason"`
	Log    string             `json:"log,omitempty"`
}

// GroupedData is the top-level object for grouped JSON. It has two arrays:
// "available" and "unavailable", each containing objects with domain + reason.
type GroupedData struct {
	Available   []GroupedDomain `json:"available"`
	Unavailable []GroupedDomain `json:"unavailable"`
}

// ExtendedGroupedData represents a grouped JSON file that may also contain
// an `unverified` list of domain records waiting to be checked.
type ExtendedGroupedData struct {
	Available   []GroupedDomain `json:"available,omitempty"`
	Unavailable []GroupedDomain `json:"unavailable,omitempty"`
	Unverified  []DomainRecord  `json:"unverified,omitempty"`
}

// checkDomainAvailability queries the WHOIS server for a single domain.
// UPDATED: now includes error details in the returned logData if dial/read fails.
func checkDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		logMsg := fmt.Sprintf("failed to connect to WHOIS: %v", err)
		return false, ReasonError, logMsg, fmt.Errorf("failed to connect to WHOIS: %w", err)
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
		readMsg := fmt.Sprintf("read error: %v", err)
		return false, ReasonError, readMsg, fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return false, ReasonError, "empty WHOIS response", fmt.Errorf("empty WHOIS response")
	}

	response := string(data)

	if strings.Contains(response, "No match for") {
		return true, ReasonNoMatch, response, nil
	}
	return false, ReasonTaken, response, nil
}

// mergeGrouped merges new grouped results into existing grouped data, deduplicating by domain.
func mergeGrouped(existing, newest GroupedData) GroupedData {
	domainsAvail := make(map[string]GroupedDomain)
	for _, gd := range existing.Available {
		domainsAvail[gd.Domain] = gd
	}
	domainsUnavail := make(map[string]GroupedDomain)
	for _, gd := range existing.Unavailable {
		domainsUnavail[gd.Domain] = gd
	}

	// Insert or update from newest
	for _, gd := range newest.Available {
		domainsAvail[gd.Domain] = gd
		delete(domainsUnavail, gd.Domain)
	}
	for _, gd := range newest.Unavailable {
		domainsUnavail[gd.Domain] = gd
		delete(domainsAvail, gd.Domain)
	}

	// Rebuild arrays
	out := GroupedData{}
	for _, rec := range domainsAvail {
		out.Available = append(out.Available, rec)
	}
	for _, rec := range domainsUnavail {
		out.Unavailable = append(out.Unavailable, rec)
	}
	return out
}

// convertArrayToGrouped turns an array of DomainRecord into GroupedData.
func convertArrayToGrouped(arr []DomainRecord) GroupedData {
	var gd GroupedData
	for _, rec := range arr {
		gDom := GroupedDomain{
			Domain: rec.Domain,
			Reason: rec.Reason,
			Log:    rec.Log,
		}

		if rec.Available {
			gd.Available = append(gd.Available, gDom)
		} else {
			gd.Unavailable = append(gd.Unavailable, gDom)
		}
	}
	return gd
}

// writeGroupedFile reads an existing grouped JSON (if any), merges new data, and writes back.
// If the existing file is an array (plain DomainRecord[]), we convert it to grouped before merging.
func writeGroupedFile(path string, newest GroupedData) error {
	if path == "" {
		return nil
	}

	existing := GroupedData{}

	// If the file exists and is not empty, parse it.
	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read grouped file: %w", err)
		}

		// First try to parse as GroupedData
		if err := json.Unmarshal(raw, &existing); err != nil {
			// If that fails, try parse as an array of DomainRecord
			var arr []DomainRecord
			if err2 := json.Unmarshal(raw, &arr); err2 == nil {
				existing = convertArrayToGrouped(arr)
			} else {
				return fmt.Errorf("parse grouped file: %w", err)
			}
		}
	}

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

// runCLIDomainArray handles the original array input logic (non-grouped or grouped output).
func runCLIDomainArray(
	whoisServer, inputPath string,
	domains []DomainRecord,
	sleep time.Duration,
	verbose, groupedOutput bool,
	outputFile string,
) int {
	groupedData := GroupedData{}

	for _, rec := range domains {
		fmt.Printf("Checking %s on %s\n", rec.Domain, whoisServer)

		avail, reason, logData, err := checkDomainAvailability(rec.Domain, whoisServer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WHOIS error for %s: %v\n", rec.Domain, err)
			avail = false
			reason = ReasonError
			logData = fmt.Sprintf("Error: %v", err)
		}

		if !groupedOutput {
			// =========== Non-Grouped Mode ===========
			rec.Available = avail
			rec.Reason = reason
			if verbose || reason == ReasonError {
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
			gd := GroupedDomain{
				Domain: rec.Domain,
				Reason: reason,
			}
			if verbose || reason == ReasonError {
				gd.Log = logData
			}

			if avail {
				groupedData.Available = append(groupedData.Available, gd)
			} else {
				groupedData.Unavailable = append(groupedData.Unavailable, gd)
			}
		}

		time.Sleep(sleep)
	}

	// Now handle final grouped output if we used --grouped-output
	if groupedOutput {
		if outputFile == "" {
			// Overwrite input file with grouped JSON
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
			if err := writeGroupedFile(outputFile, groupedData); err != nil {
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

// runCLIGroupedInput handles input that's already in the grouped JSON format with unverified domains
func runCLIGroupedInput(
	whoisServer, inputPath string,
	ext ExtendedGroupedData,
	sleep time.Duration,
	verbose, groupedOutput bool,
	outputFile string,
) int {
	// If groupedOutput was NOT specified, we force it here
	finalOutputFile := outputFile
	if !groupedOutput || outputFile == "" {
		finalOutputFile = inputPath
	}

	// Initialize arrays if they're nil
	if ext.Available == nil {
		ext.Available = []GroupedDomain{}
	}
	if ext.Unavailable == nil {
		ext.Unavailable = []GroupedDomain{}
	}

	// We'll do whois checks on the "unverified" array.
	for _, rec := range ext.Unverified {
		fmt.Printf("Checking %s on %s\n", rec.Domain, whoisServer)

		avail, reason, logData, err := checkDomainAvailability(rec.Domain, whoisServer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WHOIS error for %s: %v\n", rec.Domain, err)
			avail = false
			reason = ReasonError
			logData = fmt.Sprintf("Error: %v", err)
		}

		gd := GroupedDomain{
			Domain: rec.Domain,
			Reason: reason,
		}
		if verbose || reason == ReasonError {
			gd.Log = logData
		}

		if avail {
			ext.Available = append(ext.Available, gd)
		} else {
			ext.Unavailable = append(ext.Unavailable, gd)
		}

		time.Sleep(sleep)
	}

	// Clear out unverified after we finish checking
	ext.Unverified = nil

	out, err := json.MarshalIndent(ext, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling grouped JSON: %v\n", err)
		return 1
	}
	if err := os.WriteFile(finalOutputFile, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing grouped JSON to %s: %v\n", finalOutputFile, err)
		return 1
	}

	if finalOutputFile == inputPath {
		fmt.Println("Processed grouped input (with unverified) and overwrote original file.")
	} else {
		fmt.Println("Processed grouped input (with unverified) and wrote results to:", finalOutputFile)
	}

	return 0
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

	// Attempt to parse input as a simple array of DomainRecord.
	var domains []DomainRecord
	err = json.Unmarshal(raw, &domains)
	if err == nil {
		// Plain slice of domain records
		return runCLIDomainArray(*whoisServer, inputPath, domains, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	// If that fails, try to parse as a grouped JSON that might contain unverified.
	var ext ExtendedGroupedData
	if err2 := json.Unmarshal(raw, &ext); err2 == nil {
		return runCLIGroupedInput(*whoisServer, inputPath, ext, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	// If both fail, then it's truly invalid JSON or an unexpected format.
	fmt.Fprintf(os.Stderr, "Error parsing JSON in %s: %v\n", inputPath, err)
	return 1
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}
