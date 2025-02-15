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

// DomainRecord represents one domain entry in the JSON file.
// It includes the domain name, a log of the WHOIS response or any error,
// and whether the domain is deemed available.
type DomainRecord struct {
	Domain    string `json:"domain"`
	Log       string `json:"log,omitempty"`
	Available bool   `json:"available,omitempty"`
}

// checkDomainAvailability connects to the specified WHOIS server, sends a query
// for the given domain, and reads the server's response. It returns whether the
// domain appears available, the raw WHOIS response, and any error encountered.
func checkDomainAvailability(domain, server string) (bool, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return false, "", err
	}
	defer conn.Close()

	// Write the domain query to the WHOIS server.
	fmt.Fprintf(conn, "%s\r\n", domain)

	// Close the write side so the server sees EOF, preventing read–read deadlocks.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	// Read the entire server response.
	data, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		return false, "", fmt.Errorf("read error: %w", err)
	}
	if len(data) == 0 {
		return false, "", fmt.Errorf("no data read from server")
	}

	response := string(data)
	// Simple heuristic: if the WHOIS response contains "No match for", treat domain as available.
	if strings.Contains(response, "No match for") {
		return true, response, nil
	}
	return false, response, nil
}

// runCLI parses command-line flags and arguments, reads the JSON file,
// and for each domain performs a WHOIS availability check. The function
// returns an integer exit code: 0 for success, non-zero if any error occurs.
func runCLI(args []string) int {
	fs := flag.NewFlagSet("talia", flag.ContinueOnError)
	whoisServer := fs.String("whois", "domain:port", "WHOIS server address (host:port)")
	sleepDelay := fs.Duration("sleep", 2*time.Second, "Sleep duration between domain checks (e.g. 1s, 500ms)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: go-whois-checker [--whois=server:port] [--sleep=2s] <json-file>")
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
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		return 1
	}

	// Process each domain in the slice.
	for i, rec := range domains {
		fmt.Printf("Checking domain: %s using WHOIS server: %s\n", rec.Domain, *whoisServer)
		available, logDetails, err := checkDomainAvailability(rec.Domain, *whoisServer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking domain %s: %v\n", rec.Domain, err)
			domains[i].Log = fmt.Sprintf("Error: %v", err)
			domains[i].Available = false
		} else {
			domains[i].Log = logDetails
			domains[i].Available = available
			fmt.Printf("Domain %s available: %v\n", rec.Domain, available)
		}

		// Update the JSON file after checking the domain.
		updatedData, err := json.MarshalIndent(domains, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return 1
		}
		if err := os.WriteFile(filename, updatedData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			return 1
		}

		// Sleep for the specified duration before proceeding to the next domain.
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
