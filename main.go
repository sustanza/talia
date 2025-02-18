// Package main provides a command-line tool (Talia) that checks domain availability
// using WHOIS lookups and updates a JSON file with the results.
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

// AvailabilityReason is a short label explaining the domain's availability result.
type AvailabilityReason string

// Define constants representing the reason a domain is or is not available.
const (
	ReasonNoMatch AvailabilityReason = "NO_MATCH"
	ReasonTaken   AvailabilityReason = "TAKEN"
	ReasonError   AvailabilityReason = "ERROR"
)

// DomainRecord represents one domain entry in the JSON file.
// It includes the domain name, whether it's available, a reason code
// explaining that availability, and optionally the WHOIS log (if verbose).
type DomainRecord struct {
	Domain    string             `json:"domain"`
	Available bool               `json:"available"`
	Reason    AvailabilityReason `json:"reason,omitempty"`
	Log       string             `json:"log,omitempty"`
}

// checkDomainAvailability connects to the specified WHOIS server, sends a query
// for the given domain, and reads the server's response. It returns:
//   - bool: Whether the domain appears available
//   - AvailabilityReason: Short code (NO_MATCH, TAKEN, or ERROR)
//   - string: The raw WHOIS log data
//   - error: Any error encountered
func checkDomainAvailability(domain, server string) (bool, AvailabilityReason, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return false, ReasonError, "", fmt.Errorf("failed to connect to %s: %w", server, err)
	}
	defer conn.Close()

	// Send the domain query to the WHOIS server
	fmt.Fprintf(conn, "%s\r\n", domain)

	// Close the write side so the server sees EOF, preventing read-read deadlocks
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	// Read the entire server response
	data, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		return false, ReasonError, "", fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return false, ReasonError, "", fmt.Errorf("no data read from server")
	}

	response := string(data)

	// Simple heuristic: if the WHOIS response contains "No match for", treat domain as available
	if strings.Contains(response, "No match for") {
		return true, ReasonNoMatch, response, nil
	}

	// Otherwise, assume the domain is taken
	return false, ReasonTaken, response, nil
}

// runCLI parses command-line flags and arguments, reads the JSON file,
// and for each domain performs a WHOIS availability check. The function
// returns an integer exit code: 0 for success, non-zero if any error occurs.
func runCLI(args []string) int {
	fs := flag.NewFlagSet("talia", flag.ContinueOnError)

	// Add WHOIS server, sleep, and verbose flags
	whoisServer := fs.String("whois", "whois.verisign-grs.com:43", "WHOIS server address (host:port)")
	sleepDelay := fs.Duration("sleep", 2*time.Second, "Sleep duration between domain checks (e.g. 1s, 500ms)")
	verbose := fs.Bool("verbose", false, "Enable verbose WHOIS logs in output JSON")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

	// Must have at least one argument: <json-file>
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: talia [--whois=server:port] [--sleep=2s] [--verbose] <json-file>")
		return 1
	}
	filename := fs.Arg(0)

	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
		return 1
	}

	var domains []DomainRecord
	if err := json.Unmarshal(data, &domains); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON in %s: %v\n", filename, err)
		return 1
	}

	// Process each domain in the slice
	for i, rec := range domains {
		fmt.Printf("Checking domain: %s using WHOIS server: %s\n", rec.Domain, *whoisServer)

		available, reason, logDetails, err := checkDomainAvailability(rec.Domain, *whoisServer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking domain %s: %v\n", rec.Domain, err)
			domains[i].Available = false
			domains[i].Reason = ReasonError
			// Only store the log if verbose
			if *verbose {
				domains[i].Log = fmt.Sprintf("Error: %v", err)
			} else {
				domains[i].Log = ""
			}
		} else {
			domains[i].Available = available
			domains[i].Reason = reason
			if *verbose {
				domains[i].Log = logDetails
			} else {
				domains[i].Log = ""
			}
			fmt.Printf("Domain %s available: %v\n", rec.Domain, available)
		}

		// Update the JSON file after checking the domain
		updatedData, err := json.MarshalIndent(domains, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return 1
		}
		if err := os.WriteFile(filename, updatedData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			return 1
		}

		// Sleep for the specified duration before proceeding to the next domain
		time.Sleep(*sleepDelay)
	}

	fmt.Println("Processing complete. Updated file:", filename)
	return 0
}

// main delegates to runCLI and exits the process with runCLI's return code.
func main() {
	code := runCLI(os.Args[1:])
	os.Exit(code)
}
