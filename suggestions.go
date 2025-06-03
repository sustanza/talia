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

// Default HTTP client and base URL for the OpenAI API.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var (
	suggestionHTTPClient httpDoer = http.DefaultClient
	openAIBase                    = defaultOpenAIBase
	openAIModel                   = defaultOpenAIModel
)

// GenerateDomainSuggestions contacts the OpenAI API using structured output
// to get domain suggestions. The returned list can be used as the
// "unverified" field in an ExtendedGroupedData file.
func GenerateDomainSuggestions(apiKey, prompt string, count int) ([]DomainRecord, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

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

// writeSuggestionsFile writes the suggested domains to path in the
// ExtendedGroupedData format.
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
	return os.WriteFile(path, b, 0644)
}
