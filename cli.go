package talia

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "time"
)

// RunCLIDomainArray processes an array of domain records, checking each domain's availability
// and updating the results either in-place (non-grouped mode) or as grouped output.
// In non-grouped mode, it updates each domain's availability status in the original array
// and writes back to the input file after each check. In grouped mode, it categorizes
// domains into available and unavailable groups.
//
// Parameters:
//   - whoisServer: WHOIS server address in "host:port" format
//   - inputPath: path to the input JSON file (used for writing results in non-grouped mode)
//   - domains: array of domain records to check
//   - sleep: delay between consecutive WHOIS queries to avoid rate limiting
//   - verbose: if true, includes raw WHOIS responses in the output
//   - groupedOutput: if true, organizes results into available/unavailable groups
//   - outputFile: destination file for grouped output (if empty, overwrites input file)
//
// Returns an exit code: 0 for success, 1 for errors.
//
//nolint:gocognit // This function orchestrates the main domain checking workflow.
func RunCLIDomainArray(
    whoisServer, inputPath string,
    domains []DomainRecord,
    sleep time.Duration,
    verbose, groupedOutput bool,
    outputFile string,
) int {
    groupedData := GroupedData{}

	for _, rec := range domains {
		fmt.Printf("Checking %s on %s\n", rec.Domain, whoisServer)

		avail, reason, logData, err := CheckDomainAvailability(rec.Domain, whoisServer)
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
			if err := os.WriteFile(inputPath, out, 0644); err != nil { //nolint:gosec // JSON files don't contain secrets
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
            // Overwrite/merge into input file using grouped file merge semantics
            if err := WriteGroupedFile(inputPath, groupedData); err != nil {
                fmt.Fprintf(os.Stderr, "Error writing grouped JSON to %s: %v\n", inputPath, err)
                return 1
            }
            fmt.Println("Processing complete in grouped-output mode (overwrote input).")
        } else {
            if err := WriteGroupedFile(outputFile, groupedData); err != nil {
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

// RunCLIGroupedInput processes ExtendedGroupedData that contains already categorized domains
// and a list of unverified domains. It checks all unverified domains and updates their status,
// moving them to the appropriate available/unavailable category. This function is designed
// to work with the output from domain suggestion tools or previous partial checks.
//
// Parameters:
//   - whoisServer: WHOIS server address in "host:port" format
//   - inputPath: path to the input JSON file (used for error messages)
//   - ext: ExtendedGroupedData containing available, unavailable, and unverified domains
//   - sleep: delay between consecutive WHOIS queries
//   - verbose: if true, includes raw WHOIS responses in the output
//   - groupedOutput: if true, writes to a separate output file; otherwise overwrites input
//   - outputFile: destination file for results (if empty and groupedOutput is true, uses inputPath)
//
// Returns an exit code: 0 for success, 1 for errors.
func RunCLIGroupedInput(
    whoisServer, inputPath string,
    ext ExtendedGroupedData,
    sleep time.Duration,
    verbose, groupedOutput bool,
    outputFile string,
) int {
    // If groupedOutput was NOT specified, we force it here
    // Determine destination: if no separate output file provided, overwrite inputPath.
    finalOutputFile := outputFile
    if !groupedOutput || outputFile == "" {
        finalOutputFile = inputPath
    }

    // Initialize arrays if they're nil to ensure stable JSON shape.
    if ext.Available == nil {
        ext.Available = []GroupedDomain{}
    }
    if ext.Unavailable == nil {
        ext.Unavailable = []GroupedDomain{}
    }

	// We'll do whois checks on the "unverified" array.
	for _, rec := range ext.Unverified {
		fmt.Printf("Checking %s on %s\n", rec.Domain, whoisServer)

		avail, reason, logData, err := CheckDomainAvailability(rec.Domain, whoisServer)
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

    if groupedOutput && outputFile != "" {
        // Merge into separate grouped output file
        gd := GroupedData{Available: ext.Available, Unavailable: ext.Unavailable}
        if err := WriteGroupedFile(outputFile, gd); err != nil {
            fmt.Fprintf(os.Stderr, "Error writing grouped file: %v\n", err)
            return 1
        }
        fmt.Println("Processed grouped input (with unverified) and wrote results to:", outputFile)
    } else {
        // Overwrite input with updated ExtendedGroupedData
        out, err := json.MarshalIndent(ext, "", "  ")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error marshaling grouped JSON: %v\n", err)
            return 1
        }
        if err := os.WriteFile(finalOutputFile, out, 0644); err != nil { //nolint:gosec // JSON files don't contain secrets
            fmt.Fprintf(os.Stderr, "Error writing grouped JSON to %s: %v\n", finalOutputFile, err)
            return 1
        }
        fmt.Println("Processed grouped input (with unverified) and overwrote original file.")
    }

	return 0
}

// RunCLI is the main entry point for the Talia command-line interface.
// It parses command-line arguments, validates inputs, and orchestrates the appropriate
// processing mode based on the input file format and flags provided.
//
// cmd/talia/main.go provides the small entrypoint that calls this function.
//
// The function supports multiple modes of operation:
//   - Standard mode: processes an array of domains from a JSON file
//   - Grouped mode: processes domains organized into available/unavailable/unverified categories
//   - Suggestion mode: generates domain suggestions using OpenAI API (requires OPENAI_API_KEY)
//
// Command-line flags:
//   - --whois: WHOIS server address (required for domain checks)
//   - --sleep: delay between checks (default 2s)
//   - --verbose: include raw WHOIS responses in output
//   - --grouped-output: organize results into available/unavailable groups
//   - --output-file: destination file for grouped output
//   - --suggest: number of domain suggestions to generate
//   - --prompt: custom prompt for domain suggestions
//   - --model: OpenAI model to use for suggestions
//
// Returns an exit code: 0 for success, 1 for errors.
func RunCLI(args []string) int {
	fs := flag.NewFlagSet("talia", flag.ContinueOnError)
	whoisServer := fs.String("whois", "", "WHOIS server, e.g. whois.verisign-grs.com:43 (required)")
	sleep := fs.Duration("sleep", 2*time.Second, "Time to sleep between domain checks (default 2s)")
	verbose := fs.Bool("verbose", false, "Include WHOIS log in 'log' field even for successful checks")
	groupedOutput := fs.Bool("grouped-output", false, "Enable grouped output (JSON object with 'available','unavailable')")
	outputFile := fs.String("output-file", "", "Path to grouped output file (if set, input file remains unmodified)")
	suggest := fs.Int("suggest", 0, "Number of domain suggestions to generate (if >0, no WHOIS checks are run)")
	prompt := fs.String("prompt", "", "Optional prompt to influence domain suggestions")
	model := fs.String("model", defaultOpenAIModel, "OpenAI model to use for suggestions")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

    // Avoid mutating package-level state: pass model via options.

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s --whois=<server:port> [--sleep=2s] [--verbose] [--grouped-output] [--output-file=path] <json-file>\n", fs.Name())
		return 1
	}
    if *suggest > 0 {
        list, err := GenerateDomainSuggestionsWithContext(
            context.Background(),
            os.Getenv("OPENAI_API_KEY"),
            *prompt,
            *suggest,
            SuggestOptions{Model: *model},
        )
        if err != nil {
            fmt.Fprintln(os.Stderr, "Error generating suggestions:", err)
            return 1
        }
        if err := writeSuggestionsFile(fs.Arg(0), list); err != nil {
            fmt.Fprintln(os.Stderr, "Error writing suggestions file:", err)
            return 1
        }
        fmt.Println("Wrote domain suggestions to", fs.Arg(0))
        return 0
    }

	if *whoisServer == "" {
		fmt.Fprintln(os.Stderr, "Error: --whois=<server:port> is required")
		return 1
	}

    inputPath := fs.Arg(0)
    // Validate input path before read
    if fi, err := os.Stat(inputPath); err != nil || fi.IsDir() {
        fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
        return 1
    }
    raw, err := os.ReadFile(inputPath) //nolint:gosec // User-provided path; validated above
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
        return 1
    }

	// Attempt to parse input as a simple array of DomainRecord.
	var domains []DomainRecord
	err = json.Unmarshal(raw, &domains)
	if err == nil {
		// Plain slice of domain records
		return RunCLIDomainArray(*whoisServer, inputPath, domains, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	// If that fails, try to parse as a grouped JSON that might contain unverified.
	var ext ExtendedGroupedData
	if err2 := json.Unmarshal(raw, &ext); err2 == nil {
		return RunCLIGroupedInput(*whoisServer, inputPath, ext, *sleep, *verbose, *groupedOutput, *outputFile)
	}

	// If both fail, then it's truly invalid JSON or an unexpected format.
	fmt.Fprintf(os.Stderr, "Error parsing JSON in %s: %v\n", inputPath, err)
	return 1
}
