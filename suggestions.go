package talia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	openAIBase                    = "https://api.openai.com/v1"
)

// GenerateDomainSuggestions contacts the OpenAI API using structured output
// to get domain suggestions. The returned list can be used as the
// "unverified" field in an ExtendedGroupedData file.
func GenerateDomainSuggestions(apiKey, prompt string, count int) ([]DomainRecord, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	ctx := context.Background()

	// Build minimal JSON schema for structured output.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"unverified": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":       "object",
					"properties": map[string]any{"domain": map[string]any{"type": "string"}},
					"required":   []string{"domain"},
				},
			},
		},
		"required":             []string{"unverified"},
		"additionalProperties": false,
	}
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	body := map[string]any{
		"model": "gpt-4o", // default model name
		"messages": []map[string]string{
			{"role": "system", "content": "You generate domain name ideas."},
			{"role": "user", "content": fmt.Sprintf("%s Return %d unique domain suggestions in the 'unverified' array.", prompt, count)},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "suggestionSchema",
				"schema": string(schemaBytes),
				"strict": true,
			},
		},
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai status %s", resp.Status)
	}

	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
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
	if err := json.Unmarshal([]byte(openaiResp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("unmarshal structured output: %w", err)
	}
	return out.Unverified, nil
}

// writeSuggestionsFile writes the suggested domains to path in the
// ExtendedGroupedData format.
func writeSuggestionsFile(path string, list []DomainRecord) error {
	data := ExtendedGroupedData{Unverified: list}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
