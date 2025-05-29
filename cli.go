// Package main implements the command line logic for Talia.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// runCLIDomainArray handles the original array input logic.
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
			rec.Available = avail
			rec.Reason = reason
			if verbose || reason == ReasonError {
				rec.Log = logData
			} else {
				rec.Log = ""
			}
			replaceDomain(domains, rec)

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

	if groupedOutput {
		if outputFile == "" {
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
		fmt.Println("Processing complete. Updated file:", inputPath)
	}

	return 0
}

// runCLIGroupedInput handles grouped JSON input with an unverified list.
func runCLIGroupedInput(
	whoisServer, inputPath string,
	ext ExtendedGroupedData,
	sleep time.Duration,
	verbose, groupedOutput bool,
	outputFile string,
) int {
	finalOutputFile := outputFile
	if !groupedOutput || outputFile == "" {
		finalOutputFile = inputPath
	}

	if ext.Available == nil {
		ext.Available = []GroupedDomain{}
	}
	if ext.Unavailable == nil {
		ext.Unavailable = []GroupedDomain{}
	}

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

// runCLI is the entry point for CLI execution.
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

	var domains []DomainRecord
	err = json.Unmarshal(raw, &domains)
	if err == nil {
		return runCLIDomainArray(*whoisServer, inputPath, domains, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	var ext ExtendedGroupedData
	if err2 := json.Unmarshal(raw, &ext); err2 == nil {
		return runCLIGroupedInput(*whoisServer, inputPath, ext, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	fmt.Fprintf(os.Stderr, "Error parsing JSON in %s: %v\n", inputPath, err)
	return 1
}
