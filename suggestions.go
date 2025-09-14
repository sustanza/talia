package talia

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "time"
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
    suggestionHTTPClient httpDoer = &http.Client{Timeout: 30 * time.Second}
    // openAIBase is the base URL for the OpenAI API endpoint.
    openAIBase = defaultOpenAIBase
    // openAIModel specifies which OpenAI model to use for generating suggestions.
    openAIModel = defaultOpenAIModel
)

// Legacy note: older code paths relied on mutable package-level state (HTTP client,
// base URL, model). An options-based API is provided below to avoid global mutation
// in callers (e.g., CLI). The legacy function remains for compatibility.

// SuggestOptions configures suggestion generation without relying on globals.
type SuggestOptions struct {
    Model      string
    BaseURL    string
    HTTPClient httpDoer
}

type chatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}
type functionSpec struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  map[string]any         `json:"parameters,omitempty"`
}
type functionCallSpec struct { Name string `json:"name"` }
type chatCompletionRequest struct {
    Model        string           `json:"model"`
    Messages     []chatMessage    `json:"messages"`
    Functions    []functionSpec   `json:"functions"`
    FunctionCall functionCallSpec `json:"function_call"`
}

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
// GenerateDomainSuggestionsWithContext performs suggestion generation with explicit context and options.
func GenerateDomainSuggestionsWithContext(ctx context.Context, apiKey, prompt string, count int, opt SuggestOptions) ([]DomainRecord, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("OPENAI_API_KEY is not set")
    }
    if count <= 0 {
        return nil, fmt.Errorf("count must be > 0")
    }

    if opt.Model == "" { opt.Model = openAIModel }
    if opt.BaseURL == "" { opt.BaseURL = openAIBase }
    hc := opt.HTTPClient
    if hc == nil { hc = suggestionHTTPClient }

    // Function schema (JSON Schema fragment) kept as map for flexibility
    fnParams := map[string]any{
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
    }
    reqBody := chatCompletionRequest{
        Model: opt.Model,
        Messages: []chatMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: fmt.Sprintf(userPromptTemplate, prompt, count)},
        },
        Functions: []functionSpec{{
            Name:        functionName,
            Description: functionDesc,
            Parameters:  fnParams,
        }},
        FunctionCall: functionCallSpec{Name: functionName},
    }
    payload, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, opt.BaseURL+"/chat/completions", bytes.NewReader(payload))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := hc.Do(req)
    if err != nil {
        return nil, fmt.Errorf("openai request: %w", err)
    }
    defer func() { _ = resp.Body.Close() }()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("openai status %s", resp.Status)
    }

    var openaiResp openAIChatResponse
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

// Backward-compatible wrapper using default context and options.
func GenerateDomainSuggestions(apiKey, prompt string, count int) ([]DomainRecord, error) {
    return GenerateDomainSuggestionsWithContext(context.Background(), apiKey, prompt, count, SuggestOptions{})
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
    // If the destination exists and contains grouped JSON, merge unverified suggestions
    if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
        if fi.IsDir() {
            return fmt.Errorf("write suggestions: %s is a directory", path)
        }
        raw, err := os.ReadFile(path)
        if err == nil {
            // Try ExtendedGroupedData first
            var existingExt ExtendedGroupedData
            if json.Unmarshal(raw, &existingExt) == nil {
                // Deduplicate by domain
                seen := make(map[string]struct{}, len(existingExt.Unverified))
                for _, r := range existingExt.Unverified {
                    seen[r.Domain] = struct{}{}
                }
                merged := existingExt.Unverified[:0]
                merged = append(merged, existingExt.Unverified...)
                for _, r := range pruned {
                    if _, ok := seen[r.Domain]; !ok {
                        seen[r.Domain] = struct{}{}
                        merged = append(merged, r)
                    }
                }
                existingExt.Unverified = merged
                data = existingExt
            } else {
                // Try GroupedData and convert to ExtendedGroupedData
                var existingG GroupedData
                if json.Unmarshal(raw, &existingG) == nil {
                    data.Available = existingG.Available
                    data.Unavailable = existingG.Unavailable
                }
            }
        }
    } else if err == nil && fi.IsDir() {
        return fmt.Errorf("write suggestions: %s is a directory", path)
    }
    b, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }
    // Atomic write similar to grouped.go
    dir := filepath.Dir(path)
    base := filepath.Base(path)
    tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
    if err != nil {
        return err
    }
    tmpName := tmp.Name()
    if _, err := tmp.Write(b); err != nil {
        _ = tmp.Close()
        _ = os.Remove(tmpName)
        return err
    }
    if err := tmp.Close(); err != nil {
        _ = os.Remove(tmpName)
        return err
    }
    if err := os.Rename(tmpName, path); err != nil {
        _ = os.Remove(tmpName)
        return err
    }
    return nil
}

// openAIChatResponse models the subset of OpenAI chat response we use.
type openAIChatResponse struct {
    Choices []struct {
        Message struct {
            FunctionCall struct {
                Name      string `json:"name"`
                Arguments string `json:"arguments"`
            } `json:"function_call"`
        } `json:"message"`
    } `json:"choices"`
}
