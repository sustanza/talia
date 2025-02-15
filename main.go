// Package main provides a command-line tool that checks domain availability
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

// DomainRecord represents one domain entry in the JSON file. It includes
// the domain name, a log of the WHOIS response or any error, and whether
// the domain is deemed available.
type DomainRecord struct {
	Domain    string `json:"domain"`
	Log       string `json:"log,omitempty"`
	Available bool   `json:"available,omitempty"`
}

// checkDomainAvailability connects to the specified WHOIS server, sends a query
// for the given domain, and attempts to read the server's response. It returns
// a boolean indicating availability, the raw WHOIS data (or error message),
// and an error if the lookup fails to complete normally.
func checkDomainAvailability(domain, server string) (bool, string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return false, "", err
	}
	defer conn.Close()

	// Send the domain query to the WHOIS server.
	fmt.Fprintf(conn, "%s\r\n", domain)

	// Close the write side so the server sees EOF, preventing a potential read–read deadlock.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	// Read the entire response from the server.
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

// runCLI parses command-line flags and arguments, reads the JSON file of
// domains, and for each domain performs a WHOIS availability check. It then
// writes updated results back to the file. The function returns an integer
// exit code: 0 for success, non-zero if any error is encountered.
func runCLI(args []string) int {
	fs := flag.NewFlagSet("talia", flag.ContinueOnError)
	whoisServer := fs.String("whois", "domain:port", "WHOIS server address (host:port)")

	// Parse the provided arguments using our FlagSet.
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

	// We require at least one non-flag argument for the JSON filename.
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: go-whois-checker [--whois=server:port] <json-file>")
		return 1
	}
	filename := fs.Arg(0)

	// Read the JSON file.
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

	// Process each domain in the slice, updating the JSON file after each check.
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

		// Marshal the updated domains slice and write back to file.
		updatedData, err := json.MarshalIndent(domains, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return 1
		}
		if err := os.WriteFile(filename, updatedData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			return 1
		}

		// Briefly sleep to avoid spamming WHOIS.
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Processing complete. Updated file:", filename)
	return 0
}

// main delegates to runCLI and exits the process with runCLI's return code.
func main() {
	code := runCLI(os.Args[1:])
	os.Exit(code)
}
