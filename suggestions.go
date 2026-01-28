package talia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// validDomainLabel matches a valid domain label: alphanumeric, may contain hyphens
// but cannot start or end with a hyphen.
var validDomainLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

const (
	defaultOpenAIBase      = "https://api.openai.com/v1"
	systemPrompt           = "You generate domain name ideas. All domain names must end with .com. Do not return any domain without .com."
	userPromptTemplate     = "%s Return %d unique domain suggestions in the 'unverified' array. Each domain must end with .com. Do not return any domain without .com."
	userPromptWithExcludes = "%s Return %d unique domain suggestions in the 'unverified' array. Each domain must end with .com. Do not return any domain without .com. Do NOT suggest any of these existing domains: %s"
	defaultOpenAIModel     = "gpt-5-mini"
	functionName           = "suggest_domains"
	functionDesc           = "Generate domain name ideas."
)

// suggestionSchema defines the JSON structure returned by OpenAI when
// generating domain suggestions. It matches the ExtendedGroupedData
// format used by Talia so the suggestions can be fed back into the
// existing checking workflow.
type suggestionSchema struct {
	Unverified []DomainRecord `json:"unverified"`
}

// httpDoer abstracts HTTP client for testing.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Test hooks for integration tests that exercise RunCLI.
// These are not exported and should only be set by tests.
var (
	testHTTPClient httpDoer
	testBaseURL    string
)

// GenerateDomainSuggestions contacts the OpenAI API using structured output
// to get domain suggestions. The returned list can be used as the
// "unverified" field in an ExtendedGroupedData file. If existingDomains is
// provided, the AI is instructed to avoid suggesting those domains.
func GenerateDomainSuggestions(apiKey, prompt string, count int, model, baseURL string, existingDomains []string) ([]DomainRecord, error) {
	client := httpDoer(http.DefaultClient)
	if testHTTPClient != nil {
		client = testHTTPClient
	}
	if testBaseURL != "" {
		baseURL = testBaseURL
	}
	return generateSuggestions(apiKey, prompt, count, model, client, baseURL, existingDomains)
}

// generateSuggestions is the internal implementation that accepts dependencies
// as parameters, enabling parallel tests without shared mutable state.
func generateSuggestions(apiKey, prompt string, count int, model string, client httpDoer, baseURL string, existingDomains []string) ([]DomainRecord, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	ctx := context.Background()

	// Define the tool schema for OpenAI tool calling (modern API format)
	tools := []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        functionName,
				"description": functionDesc,
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"unverified": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"domain": map[string]any{"type": "string"},
								},
								"required": []string{"domain"},
							},
						},
					},
					"required":             []string{"unverified"},
					"additionalProperties": false,
				},
			},
		},
	}

	// Build user prompt, including existing domains to avoid if any
	var userContent string
	if len(existingDomains) > 0 {
		userContent = fmt.Sprintf(userPromptWithExcludes, prompt, count, strings.Join(existingDomains, ", "))
	} else {
		userContent = fmt.Sprintf(userPromptTemplate, prompt, count)
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"tools": tools,
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]string{"name": functionName},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai status %s", resp.Status)
	}

	var openaiResp struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}
	if len(openaiResp.Choices[0].Message.ToolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls returned")
	}

	var out suggestionSchema
	if err := json.Unmarshal([]byte(openaiResp.Choices[0].Message.ToolCalls[0].Function.Arguments), &out); err != nil {
		return nil, fmt.Errorf("unmarshal structured output: %w", err)
	}
	return out.Unverified, nil
}

// normalizeDomain cleans up and validates a domain name.
// Returns empty string if the domain is invalid.
func normalizeDomain(domain string) string {
	d := strings.TrimSpace(strings.ToLower(domain))

	// Strip repeated .com suffixes
	for strings.HasSuffix(d, ".com.com") {
		d = strings.TrimSuffix(d, ".com")
	}

	// Remove double dots
	for strings.Contains(d, "..") {
		d = strings.ReplaceAll(d, "..", ".")
	}

	// Must end with .com
	if !strings.HasSuffix(d, ".com") {
		return ""
	}

	// Basic format validation: name.com
	parts := strings.Split(d, ".")
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}

	// Validate the label contains only valid characters (letters, digits, hyphens)
	// and doesn't start or end with a hyphen
	if !validDomainLabel.MatchString(parts[0]) {
		return ""
	}

	return d
}

// writeSuggestionsFile writes the suggested domains to path in the
// ExtendedGroupedData format. If the file already exists, it merges
// new suggestions with existing data and deduplicates.
func writeSuggestionsFile(path string, list []DomainRecord) error {
	// Read existing file if it exists
	var existing ExtendedGroupedData
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &existing)
	}

	// Build set of all existing domains for deduplication
	seen := make(map[string]bool)
	for _, d := range existing.Available {
		seen[strings.ToLower(d.Domain)] = true
	}
	for _, d := range existing.Unavailable {
		seen[strings.ToLower(d.Domain)] = true
	}
	for _, d := range existing.Unverified {
		seen[strings.ToLower(d.Domain)] = true
	}

	// Add new suggestions if not already present
	for _, rec := range list {
		domain := normalizeDomain(rec.Domain)
		if domain == "" {
			continue // skip invalid
		}
		if !seen[domain] {
			seen[domain] = true
			existing.Unverified = append(existing.Unverified, DomainRecord{Domain: domain})
		}
	}

	b, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// cleanSuggestionsFile reads an existing suggestions file, normalizes all domains,
// removes invalid ones, deduplicates, and writes back. Returns count of removed domains.
func cleanSuggestionsFile(path string) (removed []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data ExtendedGroupedData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var cleaned ExtendedGroupedData

	// Process available
	for _, d := range data.Available {
		n := normalizeDomain(d.Domain)
		if n == "" {
			removed = append(removed, d.Domain)
			continue
		}
		if !seen[n] {
			seen[n] = true
			cleaned.Available = append(cleaned.Available, GroupedDomain{Domain: n, Reason: d.Reason, Log: d.Log})
		}
	}

	// Process unavailable
	for _, d := range data.Unavailable {
		n := normalizeDomain(d.Domain)
		if n == "" {
			removed = append(removed, d.Domain)
			continue
		}
		if !seen[n] {
			seen[n] = true
			cleaned.Unavailable = append(cleaned.Unavailable, GroupedDomain{Domain: n, Reason: d.Reason, Log: d.Log})
		}
	}

	// Process unverified
	for _, d := range data.Unverified {
		n := normalizeDomain(d.Domain)
		if n == "" {
			removed = append(removed, d.Domain)
			continue
		}
		if !seen[n] {
			seen[n] = true
			cleaned.Unverified = append(cleaned.Unverified, DomainRecord{Domain: n})
		}
	}

	out, err := json.MarshalIndent(cleaned, "", "  ")
	if err != nil {
		return removed, err
	}
	return removed, os.WriteFile(path, out, 0644)
}

// mergeFiles merges domains from multiple input files into outputFile, deduplicating.
// Returns the total number of unique domains in the merged result.
func mergeFiles(outputFile string, inputFiles []string) (int, error) {
	var merged ExtendedGroupedData
	seen := make(map[string]bool)

	// Helper to add domains from a source to the merged result
	mergeSource := func(source ExtendedGroupedData) {
		for _, d := range source.Available {
			domain := normalizeDomain(d.Domain)
			if domain == "" {
				continue
			}
			if !seen[domain] {
				seen[domain] = true
				merged.Available = append(merged.Available, GroupedDomain{Domain: domain, Reason: d.Reason, Log: d.Log})
			}
		}
		for _, d := range source.Unavailable {
			domain := normalizeDomain(d.Domain)
			if domain == "" {
				continue
			}
			if !seen[domain] {
				seen[domain] = true
				merged.Unavailable = append(merged.Unavailable, GroupedDomain{Domain: domain, Reason: d.Reason, Log: d.Log})
			}
		}
		for _, d := range source.Unverified {
			domain := normalizeDomain(d.Domain)
			if domain == "" {
				continue
			}
			if !seen[domain] {
				seen[domain] = true
				merged.Unverified = append(merged.Unverified, DomainRecord{Domain: domain})
			}
		}
	}

	// Read and merge all input files
	for _, inputFile := range inputFiles {
		raw, err := os.ReadFile(inputFile)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", inputFile, err)
		}
		var source ExtendedGroupedData
		if err := json.Unmarshal(raw, &source); err != nil {
			return 0, fmt.Errorf("parsing %s: %w", inputFile, err)
		}
		mergeSource(source)
	}

	totalDomains := len(merged.Available) + len(merged.Unavailable) + len(merged.Unverified)

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return totalDomains, err
	}
	return totalDomains, os.WriteFile(outputFile, out, 0644)
}

// exportAvailableDomains reads an input file and exports all available domains
// to a plain text file (one domain per line). Returns the number of domains exported.
func exportAvailableDomains(inputFile, outputFile string) (int, error) {
	raw, err := os.ReadFile(inputFile)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", inputFile, err)
	}

	var data ExtendedGroupedData
	if err := json.Unmarshal(raw, &data); err != nil {
		return 0, fmt.Errorf("parsing %s: %w", inputFile, err)
	}

	var lines []string
	for _, d := range data.Available {
		lines = append(lines, d.Domain)
	}

	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}

	if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
		return 0, fmt.Errorf("writing %s: %w", outputFile, err)
	}

	return len(lines), nil
}

// readExistingDomains reads all domains from an existing suggestions file
// for use in avoiding duplicates when generating new suggestions.
func readExistingDomains(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var data ExtendedGroupedData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}

	var domains []string
	for _, d := range data.Available {
		domains = append(domains, d.Domain)
	}
	for _, d := range data.Unavailable {
		domains = append(domains, d.Domain)
	}
	for _, d := range data.Unverified {
		domains = append(domains, d.Domain)
	}
	return domains
}
