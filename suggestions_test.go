package talia

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"a.com\"}]}"}}]}}]}`)
	}))
	defer srv.Close()

	got, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err != nil {
		t.Fatalf("generateSuggestions returned error: %v", err)
	}
	if len(got) != 1 || got[0].Domain != "a.com" {
		t.Fatalf("unexpected suggestions: %+v", got)
	}
}

func TestGenerateDomainSuggestionsHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

func TestRunCLISuggest(t *testing.T) {
	// Integration test: cannot be parallel due to test hooks and env vars
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"b.com\"}]}"}}]}}]}`)
	}))
	defer srv.Close()

	testHTTPClient = fakeHTTPClient{srv}
	testBaseURL = srv.URL
	t.Cleanup(func() {
		testHTTPClient = nil
		testBaseURL = ""
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

	stdout, _ := captureOutput(t, func() {
		code := RunCLI([]string{"--suggest=1", tmp.Name()})
		if code != 0 {
			t.Errorf("expected exit 0, got %d", code)
		}
	})

	// Check for success message
	if !strings.Contains(stdout, "suggestions total") {
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
	// Integration test: cannot be parallel due to test hooks and env vars
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
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"c.com\"}]}"}}]}}]}`)
	}))
	defer srv.Close()

	testHTTPClient = fakeHTTPClient{srv}
	testBaseURL = srv.URL
	t.Cleanup(func() {
		testHTTPClient = nil
		testBaseURL = ""
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
		code := RunCLI([]string{"--suggest=1", "--model=my-model", tmp.Name()})
		if code != 0 {
			t.Errorf("expected exit 0, got %d", code)
		}
	})

	if gotModel != "my-model" {
		t.Fatalf("server received model %q, want %q", gotModel, "my-model")
	}
}

func TestGenerateDomainSuggestionsNoAPIKey(t *testing.T) {
	t.Parallel()
	_, err := generateSuggestions("", "", 1, "gpt-4o", http.DefaultClient, "", nil)
	if err == nil || err.Error() != "OPENAI_API_KEY is not set" {
		t.Fatalf("expected OPENAI_API_KEY error, got %v", err)
	}
}

type errClient struct{}

func (errClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

func TestGenerateDomainSuggestionsRequestError(t *testing.T) {
	t.Parallel()
	_, err := generateSuggestions("key", "", 1, "gpt-4o", errClient{}, "http://localhost", nil)
	if err == nil || !strings.Contains(err.Error(), "openai request") {
		t.Fatalf("expected openai request error, got %v", err)
	}
}

func TestGenerateDomainSuggestionsDecodeError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not-json")
	}))
	defer srv.Close()
	_, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestGenerateDomainSuggestionsNoChoices(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()
	_, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("expected no choices error, got %v", err)
	}
}

func TestGenerateDomainSuggestionsNoToolCalls(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[]}}]}`)
	}))
	defer srv.Close()
	_, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "no tool calls") {
		t.Fatalf("expected no tool calls error, got %v", err)
	}
}

func TestGenerateDomainSuggestionsUnmarshalError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"suggest_domains","arguments":"not-json"}}]}}]}`)
	}))
	defer srv.Close()
	_, err := generateSuggestions("key", "", 1, "gpt-4o", fakeHTTPClient{srv}, srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "unmarshal structured output") {
		t.Fatalf("expected unmarshal error, got %v", err)
	}
}

func TestWriteSuggestionsFile_Error(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "dir")
	if err != nil {
		t.Fatal(err)
	}
	defer helperRemoveAll(t, dir)
	err = writeSuggestionsFile(dir, []DomainRecord{{Domain: "a.com"}})
	if err == nil {
		t.Fatal("expected error writing to directory, got nil")
	}
}

func TestCleanTextFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.txt")

	content := "example.com\nINVALID DOMAIN.com\nexample.com\ntest123.com\n\n# comment\n-bad-.com\ntest123.com\ngood-name.com\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := cleanTextFile(path)
	if err != nil {
		t.Fatalf("cleanTextFile error: %v", err)
	}

	// Should have removed 2 invalid domains
	if len(removed) != 2 {
		t.Errorf("expected 2 removed, got %d: %v", len(removed), removed)
	}

	// Read back and verify
	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	// Should have 3 unique valid domains
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
	}
}

func TestNormalizeDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		// Valid domains
		{"example.com", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"  example.com  ", "example.com"},
		{"my-domain.com", "my-domain.com"},
		{"test123.com", "test123.com"},
		{"a1b2c3.com", "a1b2c3.com"},

		// Repeated .com suffixes
		{"example.com.com", "example.com"},
		{"example.com.com.com", "example.com"},
		{"Lordboard..com.com.com", "lordboard.com"},

		// Double dots
		{"example..com", "example.com"},
		{"ex..am..ple.com", ""}, // becomes ex.am.ple.com which has subdomains, invalid

		// Invalid: wrong TLD
		{"example.net", ""},
		{"example.org", ""},

		// Invalid: no name
		{".com", ""},
		{"", ""},

		// Invalid: subdomains (not simple name.com)
		{"sub.example.com", ""},
		{"a.b.com", ""},

		// Invalid: spaces in domain name
		{"flora board.com", ""},
		{"hello board.com", ""},
		{"my domain.com", ""},

		// Invalid: special characters
		{"test_domain.com", ""},
		{"test@domain.com", ""},
		{"test!domain.com", ""},
		{"test#domain.com", ""},

		// Invalid: hyphen at start or end
		{"-example.com", ""},
		{"example-.com", ""},
		{"-test-.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDomain(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// suggestAPICalls records the requests received by a fake suggestion API.
type suggestAPICalls struct {
	mu     sync.Mutex
	models []string
}

func (c *suggestAPICalls) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.models...)
}

// runSuggestCLI runs RunCLI against a fake suggestion API with the given env vars set
// and returns the model named in each request the API received.
func runSuggestCLI(t *testing.T, env map[string]string, args ...string) []string {
	t.Helper()
	calls := &suggestAPICalls{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		calls.mu.Lock()
		calls.models = append(calls.models, payload.Model)
		calls.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"suggest_domains","arguments":"{\"unverified\":[{\"domain\":\"c.com\"}]}"}}]}}]}`)
	}))
	t.Cleanup(srv.Close)
	testHTTPClient = fakeHTTPClient{srv}
	testBaseURL = srv.URL
	t.Cleanup(func() {
		testHTTPClient = nil
		testBaseURL = ""
	})

	for _, k := range []string{"TALIA_MODEL", "TALIA_SUGGEST", "TALIA_SUGGEST_PARALLEL", "WHOIS_SERVER", "TALIA_LIGHTSPEED"} {
		t.Setenv(k, env[k])
	}
	t.Setenv("OPENAI_API_KEY", "key")

	file := filepath.Join(t.TempDir(), "suggestions.json")
	captureOutput(t, func() { RunCLI(append(args, file)) })
	return calls.snapshot()
}

func TestRunCLI_ExplicitFlagsBeatEnvFallbacks(t *testing.T) {
	cases := []struct {
		name         string
		env          map[string]string
		args         []string
		wantRequests int
		wantModel    string
	}{
		{"explicit default model", map[string]string{"TALIA_MODEL": "env-model"},
			[]string{"--suggest=1", "--model=" + defaultOpenAIModel}, 1, defaultOpenAIModel},
		{"model from env", map[string]string{"TALIA_MODEL": "env-model"},
			[]string{"--suggest=1"}, 1, "env-model"},
		{"explicit default parallel", map[string]string{"TALIA_SUGGEST_PARALLEL": "4"},
			[]string{"--suggest=1", "--suggest-parallel=1"}, 1, defaultOpenAIModel},
		{"parallel from env", map[string]string{"TALIA_SUGGEST_PARALLEL": "4"},
			[]string{"--suggest=1"}, 4, defaultOpenAIModel},
		{"explicit zero suggest", map[string]string{"TALIA_SUGGEST": "10"},
			[]string{"--suggest=0"}, 0, ""},
		{"suggest from env", map[string]string{"TALIA_SUGGEST": "1"},
			nil, 1, defaultOpenAIModel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			models := runSuggestCLI(t, tc.env, tc.args...)
			if len(models) != tc.wantRequests {
				t.Fatalf("API received %d requests, want %d", len(models), tc.wantRequests)
			}
			for _, m := range models {
				if m != tc.wantModel {
					t.Errorf("request used model %q, want %q", m, tc.wantModel)
				}
			}
		})
	}
}
