package talia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const (
	defaultOpenAIBase  = "https://api.openai.com/v1"
	systemPrompt       = "You generate domain name ideas. All domain names must end with .com. Do not return any domain without .com."
	userPromptTemplate = "%s Return %d unique domain suggestions in the 'unverified' array. Each domain must end with .com. Do not return any domain without .com."
	defaultOpenAIModel = "gpt-4o"
	functionName       = "suggest_domains"
	functionDesc       = "Generate domain name ideas."
)

// suggestionSchema defines the JSON structure returned by OpenAI when
// generating domain suggestions. It matches the ExtendedGroupedData
// format used by Talia so the suggestions can be fed back into the
// existing checking workflow.
type suggestionSchema struct {
	Unverified []DomainRecord `json:"unverified"`
}

// httpDoer defines the interface for HTTP client operations, allowing for easy testing
// and mocking of HTTP requests to the OpenAI API.
type httpDoer interface {
    Do(*http.Request) (*http.Response, error)
}

var (
    // suggestionHTTPClient is the HTTP client used for OpenAI API requests.
    // It can be replaced for testing purposes.
    suggestionHTTPClient httpDoer = http.DefaultClient
    // openAIBase is the base URL for the OpenAI API endpoint.
    openAIBase = defaultOpenAIBase
    // openAIModel specifies which OpenAI model to use for generating suggestions.
    openAIModel = defaultOpenAIModel
)

// TODO(sustanza): Avoid mutable package-level state (AGENTS.md Security & Configuration Tips).
//  - Inject `httpDoer`, base URL, and model via parameters or a small Config/options type.
//  - Tests should pass fakes through parameters instead of mutating globals.
//  - Ensure `http.Client` has a Timeout configured to avoid hanging requests.

// GenerateDomainSuggestions uses the OpenAI API to generate creative domain name suggestions
// based on a user-provided prompt. It leverages OpenAI's function calling feature with
// structured output to ensure suggestions are returned in the correct format. All suggested
// domains will end with .com as enforced by the system prompt.
//
// Parameters:
//   - apiKey: OpenAI API key for authentication
//   - prompt: user's description of desired domain names (e.g., "tech startup focused on AI")
//   - count: number of domain suggestions to generate
//
// Returns a slice of DomainRecord entries ready to be checked for availability,
// or an error if the API call fails or returns invalid data.
// TODO(sustanza): Consider accepting context from caller (ctx parameter)
// for cancellation/timeouts instead of using context.Background().
func GenerateDomainSuggestions(apiKey, prompt string, count int) ([]DomainRecord, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("OPENAI_API_KEY is not set")
    }
    // TODO(sustanza): Validate `count > 0` (and possibly max bound) to avoid
    // upstream requests with invalid parameters when used as a library.

    ctx := context.Background()

	// Define the function schema for OpenAI function calling
	functions := []map[string]any{
		{
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
	}

    // TODO(sustanza): Replace map[string]any payload with a typed request struct
    // to improve compile-time safety and self-documentation.
    body := map[string]any{
        "model": openAIModel,
        "messages": []map[string]string{
            {"role": "system", "content": systemPrompt},
            {"role": "user", "content": fmt.Sprintf(userPromptTemplate, prompt, count)},
        },
        "functions":     functions,
        "function_call": map[string]string{"name": functionName},
    }

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

    // TODO(sustanza): Pass base URL and model as parameters or via an options struct
    // rather than reading mutable package vars.
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIBase+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suggestionHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai status %s", resp.Status)
	}

    // TODO(sustanza): Define a top-level typed response struct in this package
    // instead of an inline anonymous struct for better reuse and clarity.
    var openaiResp struct {
        Choices []struct {
            Message struct {
                FunctionCall struct {
                    Name      string `json:"name"`
                    Arguments string `json:"arguments"`
                } `json:"function_call"`
            } `json:"message"`
        } `json:"choices"`
    }
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	var out suggestionSchema
	if err := json.Unmarshal([]byte(openaiResp.Choices[0].Message.FunctionCall.Arguments), &out); err != nil {
		return nil, fmt.Errorf("unmarshal structured output: %w", err)
	}
	return out.Unverified, nil
}

// writeSuggestionsFile saves domain suggestions to a JSON file in ExtendedGroupedData format.
// The suggestions are placed in the "unverified" field, ready to be checked for availability
// in a subsequent run. The function strips any existing availability information from the
// domain records, keeping only the domain names to ensure a clean starting state.
//
// Parameters:
//   - path: destination file path for the JSON output
//   - list: slice of DomainRecord entries containing the suggested domains
//
// Returns an error if the file write operation fails.
func writeSuggestionsFile(path string, list []DomainRecord) error {
    // Remove any fields except Domain from each DomainRecord for suggestions output
    pruned := make([]DomainRecord, 0, len(list))
    for _, rec := range list {
        pruned = append(pruned, DomainRecord{Domain: rec.Domain})
    }
    data := ExtendedGroupedData{Unverified: pruned}
    b, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }
    // TODO(sustanza): If `path` already exists and contains grouped JSON, consider merging
    // new unverified suggestions into existing `unverified` rather than overwriting.
    // TODO(sustanza): Perform an atomic write (temp file + rename) like grouped.go.
    return os.WriteFile(path, b, 0644) //nolint:gosec // JSON files don't contain secrets
}
