package talia

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeHTTPClient implements the Do method for testing.
type fakeHTTPClient struct{ srv *httptest.Server }

func (f fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	f.srv.Config.Handler.ServeHTTP(rr, req)
	return rr.Result(), nil
}

// TestGenerateDomainSuggestionsSuccess verifies we parse suggestions correctly.
func TestGenerateDomainSuggestionsSuccess(t *testing.T) {
	// fake OpenAI server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// return structured output JSON
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"function_call":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"a.com\"}]}"}}}]}`)
	}))
	defer srv.Close()

	suggestionHTTPClient = fakeHTTPClient{srv}
	openAIBase = srv.URL
	t.Cleanup(func() {
		suggestionHTTPClient = http.DefaultClient
		openAIBase = "https://api.openai.com/v1"
	})

	got, err := GenerateDomainSuggestions("key", "", 1)
	if err != nil {
		t.Fatalf("GenerateDomainSuggestions returned error: %v", err)
	}
	if len(got) != 1 || got[0].Domain != "a.com" {
		t.Fatalf("unexpected suggestions: %+v", got)
	}
}

func TestGenerateDomainSuggestionsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	suggestionHTTPClient = fakeHTTPClient{srv}
	openAIBase = srv.URL
	t.Cleanup(func() {
		suggestionHTTPClient = http.DefaultClient
		openAIBase = "https://api.openai.com/v1"
	})

	_, err := GenerateDomainSuggestions("key", "", 1)
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

func TestRunCLISuggest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"function_call":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"b.com\"}]}"}}}]}`)
	}))
	defer srv.Close()

	suggestionHTTPClient = fakeHTTPClient{srv}
	openAIBase = srv.URL
	t.Cleanup(func() {
		suggestionHTTPClient = http.DefaultClient
		openAIBase = "https://api.openai.com/v1"
	})

	tmp, err := os.CreateTemp("", "sugg_*.json")
	if err != nil {
		t.Fatal(err)
	}
	err = tmp.Close()
	if err != nil {
		t.Fatalf("tmp.Close() error: %v", err)
	}
	defer helperRemove(t, tmp.Name())

	err = os.Setenv("OPENAI_API_KEY", "key")
	if err != nil {
		t.Fatalf("os.Setenv error: %v", err)
	}
	defer func() {
		err := os.Unsetenv("OPENAI_API_KEY")
		if err != nil {
			t.Fatalf("os.Unsetenv error: %v", err)
		}
	}()

	stdout, stderr := captureOutput(t, func() {
		code := RunCLI([]string{"--suggest=1", tmp.Name()})
		if code != 0 {
			t.Errorf("expected exit 0, got %d", code)
		}
	})

	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "Wrote domain suggestions") {
		t.Errorf("missing success message: %s", stdout)
	}

	raw, _ := os.ReadFile(tmp.Name())
	var out ExtendedGroupedData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(out.Unverified) != 1 || out.Unverified[0].Domain != "b.com" {
		t.Fatalf("unexpected file contents: %+v", out)
	}
}

func TestRunCLISuggestModelFlag(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		if v, ok := payload["model"].(string); ok {
			gotModel = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"function_call":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"c.com\"}]}"}}}]}`)
	}))
	defer srv.Close()

	suggestionHTTPClient = fakeHTTPClient{srv}
	openAIBase = srv.URL
	t.Cleanup(func() {
		suggestionHTTPClient = http.DefaultClient
		openAIBase = "https://api.openai.com/v1"
		openAIModel = defaultOpenAIModel
	})

	tmp, err := os.CreateTemp("", "sugg_model_*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("tmp.Close() error: %v", err)
	}
	defer helperRemove(t, tmp.Name())

	if err := os.Setenv("OPENAI_API_KEY", "key"); err != nil {
		t.Fatalf("os.Setenv error: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
			t.Fatalf("os.Unsetenv error: %v", err)
		}
	}()

	_, _ = captureOutput(t, func() {
		code := RunCLI([]string{"--suggest=1", "--openai-model=my-model", tmp.Name()})
		if code != 0 {
			t.Errorf("expected exit 0, got %d", code)
		}
	})

	if gotModel != "my-model" {
		t.Fatalf("server received model %q, want %q", gotModel, "my-model")
	}
}
