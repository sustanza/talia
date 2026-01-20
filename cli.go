// Package talia provides the core logic for checking domain availability via
// WHOIS and processing JSON domain lists.
package talia

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// checkResult holds the result of a single domain availability check.
type checkResult struct {
	Domain string
	Avail  bool
	Reason AvailabilityReason
	Log    string
}

// shouldIncludeLog determines whether to include the WHOIS log in output.
func shouldIncludeLog(verbose bool, reason AvailabilityReason) bool {
	return verbose || reason == ReasonError
}

// checkDomains performs WHOIS checks on a list of domains and returns the results.
func checkDomains(domains []string, whoisServer string, sleep time.Duration, verbose bool) []checkResult {
	results := make([]checkResult, 0, len(domains))
	prog := newProgress(len(domains))
	stats := newCheckStats()

	for _, domain := range domains {
		prog.Increment()

		avail, reason, logData, err := CheckDomainAvailability(domain, whoisServer)
		if err != nil {
			avail = false
			reason = ReasonError
			logData = fmt.Sprintf("Error: %v", err)
		}

		prog.PrintCheck(domain, avail, reason)
		stats.Record(avail, reason)

		log := ""
		if shouldIncludeLog(verbose, reason) {
			log = logData
		}

		results = append(results, checkResult{
			Domain: domain,
			Avail:  avail,
			Reason: reason,
			Log:    log,
		})

		time.Sleep(sleep)
	}

	stats.PrintSummary()
	return results
}

// RunCLIDomainArray handles the original array input logic (non-grouped or grouped output).
func RunCLIDomainArray(
	whoisServer, inputPath string,
	domains []DomainRecord,
	sleep time.Duration,
	verbose, groupedOutput bool,
	outputFile string,
) int {
	// Extract domain names for checking
	domainNames := make([]string, len(domains))
	for i := range domains {
		domainNames[i] = domains[i].Domain
	}

	results := checkDomains(domainNames, whoisServer, sleep, verbose)

	if !groupedOutput {
		// =========== Non-Grouped Mode ===========
		for i, res := range results {
			domains[i].Available = res.Avail
			domains[i].Reason = res.Reason
			domains[i].Log = res.Log
		}

		out, err := json.MarshalIndent(domains, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return 1
		}
		if err := os.WriteFile(inputPath, out, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			return 1
		}
		fmt.Println("Processing complete. Updated file:", inputPath)
	} else {
		// =========== Grouped Mode ===========
		groupedData := GroupedData{}
		for _, res := range results {
			gd := GroupedDomain{
				Domain: res.Domain,
				Reason: res.Reason,
				Log:    res.Log,
			}
			if res.Avail {
				groupedData.Available = append(groupedData.Available, gd)
			} else {
				groupedData.Unavailable = append(groupedData.Unavailable, gd)
			}
		}

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
			if err := WriteGroupedFile(outputFile, groupedData); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing grouped file: %v\n", err)
				return 1
			}
			fmt.Println("Processing complete in grouped-output mode (wrote to separate file).")
		}
	}

	return 0
}

// RunCLIGroupedInput handles input that's already in the grouped JSON format with unverified domains
func RunCLIGroupedInput(
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

	// Extract domain names for checking
	domainNames := make([]string, len(ext.Unverified))
	for i := range ext.Unverified {
		domainNames[i] = ext.Unverified[i].Domain
	}

	results := checkDomains(domainNames, whoisServer, sleep, verbose)

	for _, res := range results {
		gd := GroupedDomain{
			Domain: res.Domain,
			Reason: res.Reason,
			Log:    res.Log,
		}
		if res.Avail {
			ext.Available = append(ext.Available, gd)
		} else {
			ext.Unavailable = append(ext.Unavailable, gd)
		}
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

// RunCLI is the main entry point for Talia logic.
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
	apiBase := fs.String("api-base", "", "Base URL for OpenAI-compatible API (default: https://api.openai.com/v1)")
	fresh := fs.Bool("fresh", false, "Don't pass existing domains to AI (allows duplicates, starts fresh)")
	clean := fs.Bool("clean", false, "Clean and normalize domains in the file (removes invalid domains)")
	noVerify := fs.Bool("no-verify", false, "Skip WHOIS verification after generating suggestions")
	merge := fs.String("merge", "", "Merge domains from another file into the target file")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing flags:", err)
		return 1
	}

	// Get target file from args or env var
	targetFile := ""
	if fs.NArg() >= 1 {
		targetFile = fs.Arg(0)
	} else if envFile := os.Getenv("TALIA_FILE"); envFile != "" {
		targetFile = envFile
	}
	if targetFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <json-file> (or set TALIA_FILE env var)\n", fs.Name())
		return 1
	}
	if *clean {
		removed, err := cleanSuggestionsFile(targetFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error cleaning file:", err)
			return 1
		}
		if len(removed) > 0 {
			fmt.Printf("Removed %d invalid domains:\n", len(removed))
			for _, d := range removed {
				fmt.Printf("  - %s\n", d)
			}
		} else {
			fmt.Println("No invalid domains found.")
		}
		fmt.Println("Cleaned", targetFile)
		return 0
	}

	if *merge != "" {
		added, err := mergeFiles(targetFile, *merge)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error merging files:", err)
			return 1
		}
		fmt.Printf("Merged %d new domains from %s into %s\n", added, *merge, targetFile)
		return 0
	}

	// Use env var if --suggest not provided
	suggestCount := *suggest
	if suggestCount == 0 {
		if envSuggest := os.Getenv("TALIA_SUGGEST"); envSuggest != "" {
			if n, err := strconv.Atoi(envSuggest); err == nil {
				suggestCount = n
			}
		}
	}

	if suggestCount > 0 {
		baseURL := *apiBase
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_API_BASE")
		}
		if baseURL == "" {
			baseURL = defaultOpenAIBase
		}
		// Use env var if --prompt not provided
		promptText := *prompt
		if promptText == "" {
			promptText = os.Getenv("TALIA_PROMPT")
		}
		// Use env var if --model not provided (and not default)
		modelName := *model
		if modelName == defaultOpenAIModel {
			if envModel := os.Getenv("TALIA_MODEL"); envModel != "" {
				modelName = envModel
			}
		}
		// Read existing domains to avoid duplicates (unless --fresh is set)
		var existingDomains []string
		if !*fresh {
			existingDomains = readExistingDomains(targetFile)
		}

		spin := newSpinner("Generating suggestions...")
		spin.Start()
		list, err := GenerateDomainSuggestions(os.Getenv("OPENAI_API_KEY"), promptText, suggestCount, modelName, baseURL, existingDomains)
		spin.Stop()

		if err != nil {
			fmt.Fprintln(os.Stderr, "Error generating suggestions:", err)
			return 1
		}
		if err := writeSuggestionsFile(targetFile, list); err != nil {
			fmt.Fprintln(os.Stderr, "Error writing suggestions file:", err)
			return 1
		}
		fmt.Println("Wrote domain suggestions to", targetFile)

		// Auto-verify suggestions if --whois is provided (or env var) and --no-verify is not set
		whois := *whoisServer
		if whois == "" {
			whois = os.Getenv("WHOIS_SERVER")
		}
		if whois != "" && !*noVerify {
			fmt.Println("Verifying suggestions...")
			inputPath := targetFile
			raw, err := os.ReadFile(inputPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
				return 1
			}
			var ext ExtendedGroupedData
			if err := json.Unmarshal(raw, &ext); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", inputPath, err)
				return 1
			}
			// Use 100ms sleep for auto-verification
			verifySleep := 100 * time.Millisecond
			return RunCLIGroupedInput(whois, inputPath, ext, verifySleep, *verbose, true, "")
		}
		return 0
	}

	// Use env var if --whois not provided
	if *whoisServer == "" {
		*whoisServer = os.Getenv("WHOIS_SERVER")
	}
	if *whoisServer == "" {
		fmt.Fprintln(os.Stderr, "Error: --whois=<server:port> is required (or set WHOIS_SERVER env var)")
		return 1
	}

	inputPath := targetFile
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
