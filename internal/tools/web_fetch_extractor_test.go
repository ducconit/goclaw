package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Quality Gate Tests ---

func TestIsQualityContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"short", "hello world", false},
		{"under_100_chars", strings.Repeat("x", 99), false},
		{"100_chars_few_words", strings.Repeat("x", 100), false}, // 1 word
		{"whitespace_heavy", strings.Repeat("  a  ", 30), true},  // 30 words, >100 chars
		{"valid_paragraph", "The quick brown fox jumps over the lazy dog. This is a sample paragraph with enough words and characters to pass the quality threshold for content extraction.", true},
		{"exactly_10_words_short", "one two three four five six seven eight nine ten", false}, // <100 chars
		{"10_words_long_enough", "word1_padding word2_padding word3_padding word4_padding word5_padding word6_padding word7_padding word8_padding word9_padding word10_padding", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isQualityContent(tt.content)
			if got != tt.want {
				t.Errorf("isQualityContent(%d chars, %d words) = %v, want %v",
					len(tt.content), len(strings.Fields(tt.content)), got, tt.want)
			}
		})
	}
}

// --- Mock Extractor ---

type mockExtractor struct {
	name    string
	content string
	err     error
}

func (m *mockExtractor) Name() string { return m.name }
func (m *mockExtractor) Extract(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.content, nil
}

// qualityContent returns a string that passes isQualityContent.
func qualityContent() string {
	return "This is a sufficiently long paragraph with more than ten words and well over one hundred characters to satisfy the quality content threshold check in the extractor chain."
}

// --- ExtractorChain Tests ---

func TestExtractorChain_FirstSuccess(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "first", content: qualityContent()},
		&mockExtractor{name: "second", content: "should not reach"},
	)
	result, err := chain.Extract(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Extractor != "first" {
		t.Errorf("expected extractor 'first', got %q", result.Extractor)
	}
}

func TestExtractorChain_FirstFailsSecondSucceeds(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "first", err: fmt.Errorf("network error")},
		&mockExtractor{name: "second", content: qualityContent()},
	)
	result, err := chain.Extract(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Extractor != "second" {
		t.Errorf("expected extractor 'second', got %q", result.Extractor)
	}
}

func TestExtractorChain_FirstLowQualitySecondSucceeds(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "first", content: "too short"},
		&mockExtractor{name: "second", content: qualityContent()},
	)
	result, err := chain.Extract(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Extractor != "second" {
		t.Errorf("expected extractor 'second', got %q", result.Extractor)
	}
}

func TestExtractorChain_AllFail(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "first", err: fmt.Errorf("fail1")},
		&mockExtractor{name: "second", err: fmt.Errorf("fail2")},
	)
	_, err := chain.Extract(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error when all extractors fail")
	}
	if !strings.Contains(err.Error(), "all extractors failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExtractorChain_AllLowQuality(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "first", content: "short"},
		&mockExtractor{name: "second", content: "also short"},
	)
	_, err := chain.Extract(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error when all extractors return low quality")
	}
}

func TestExtractorChain_Empty(t *testing.T) {
	chain := NewExtractorChain()
	_, err := chain.Extract(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error with empty chain")
	}
	if !strings.Contains(err.Error(), "no extractors configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtractorChain_SingleSuccess(t *testing.T) {
	chain := NewExtractorChain(
		&mockExtractor{name: "only", content: qualityContent()},
	)
	result, err := chain.Extract(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Extractor != "only" {
		t.Errorf("expected 'only', got %q", result.Extractor)
	}
}

// --- DefuddleExtractor Tests ---

// newTestDefuddleExtractor creates a DefuddleExtractor pointing at a test server.
// The test server URL replaces the production fetch.goclaw.sh base URL.
func newTestDefuddleExtractor(serverURL string) *DefuddleExtractor {
	return newDefuddleExtractor(serverURL + "/")
}

func TestDefuddleExtractor_Success(t *testing.T) {
	expected := qualityContent()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path received is the stripped URL (no scheme)
		if r.URL.Path != "/example.com/page" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte(expected))
	}))
	defer server.Close()

	ext := newTestDefuddleExtractor(server.URL)
	result, err := ext.Extract(context.Background(), "https://example.com/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Errorf("content mismatch: got %d chars, want %d chars", len(result), len(expected))
	}
}

func TestDefuddleExtractor_URLConstruction(t *testing.T) {
	tests := []struct {
		input        string
		expectedPath string
	}{
		{"https://example.com/page", "/example.com/page"},
		{"http://example.com/page", "/example.com/page"},
		{"https://x.com/user/status/123", "/x.com/user/status/123"},
	}
	for _, tt := range tests {
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(qualityContent()))
		}))
		ext := newTestDefuddleExtractor(server.URL)
		_, _ = ext.Extract(context.Background(), tt.input)
		server.Close()

		if gotPath != tt.expectedPath {
			t.Errorf("input=%q: got path %q, want %q", tt.input, gotPath, tt.expectedPath)
		}
	}
}

func TestDefuddleExtractor_HTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ext := newTestDefuddleExtractor(server.URL)
	_, err := ext.Extract(context.Background(), "https://example.com/missing")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefuddleExtractor_HTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ext := newTestDefuddleExtractor(server.URL)
	_, err := ext.Extract(context.Background(), "https://example.com/error")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDefuddleExtractor_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// empty body
	}))
	defer server.Close()

	ext := newTestDefuddleExtractor(server.URL)
	result, err := ext.Extract(context.Background(), "https://example.com/empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty body returns successfully — the chain's quality gate will catch it
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// --- InProcessExtractor Tests ---

func TestInProcessExtractor_HTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><h1>Hello World</h1><p>This is a test paragraph with enough content to be meaningful and pass quality checks.</p></body></html>`))
	}))
	defer server.Close()

	ext := NewInProcessExtractor()
	result, err := ext.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "# Hello World") {
		t.Errorf("expected markdown heading, got: %s", result)
	}
	if !strings.Contains(result, "test paragraph") {
		t.Errorf("expected paragraph content, got: %s", result)
	}
}

func TestInProcessExtractor_JSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key":"value","nested":{"a":1}}`))
	}))
	defer server.Close()

	ext := NewInProcessExtractor()
	result, err := ext.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, `"key": "value"`) {
		t.Errorf("expected pretty-printed JSON, got: %s", result)
	}
}

func TestInProcessExtractor_Markdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# Title\n\nSome content here"))
	}))
	defer server.Close()

	ext := NewInProcessExtractor()
	result, err := ext.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "# Title\n\nSome content here" {
		t.Errorf("expected markdown passthrough, got: %s", result)
	}
}

func TestInProcessExtractor_RawText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain text content"))
	}))
	defer server.Close()

	ext := NewInProcessExtractor()
	result, err := ext.Extract(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain text content" {
		t.Errorf("expected raw text, got: %s", result)
	}
}

func TestInProcessExtractor_EmptyHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><script>app.init()</script></body></html>`))
	}))
	defer server.Close()

	ext := NewInProcessExtractor()
	_, err := ext.Extract(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for empty HTML extraction")
	}
	if !strings.Contains(err.Error(), "no content extracted") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- WebFetchTool Config Toggle Tests ---

func TestWebFetchTool_DefuddleEnabledChain(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{DefuddleEnabled: true})
	tool.mu.RLock()
	chain := tool.chain
	tool.mu.RUnlock()

	if chain == nil {
		t.Fatal("chain should not be nil when defuddle enabled")
	}
	if len(chain.extractors) != 2 {
		t.Errorf("expected 2 extractors (defuddle + inprocess), got %d", len(chain.extractors))
	}
	if chain.extractors[0].Name() != "defuddle" {
		t.Errorf("first extractor should be 'defuddle', got %q", chain.extractors[0].Name())
	}
	if chain.extractors[1].Name() != "html-to-markdown" {
		t.Errorf("second extractor should be 'html-to-markdown', got %q", chain.extractors[1].Name())
	}
}

func TestWebFetchTool_DefuddleDisabledChain(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{DefuddleEnabled: false})
	tool.mu.RLock()
	chain := tool.chain
	tool.mu.RUnlock()

	if chain == nil {
		t.Fatal("chain should not be nil")
	}
	if len(chain.extractors) != 1 {
		t.Errorf("expected 1 extractor (inprocess only), got %d", len(chain.extractors))
	}
	if chain.extractors[0].Name() != "html-to-markdown" {
		t.Errorf("expected 'html-to-markdown', got %q", chain.extractors[0].Name())
	}
}

func TestWebFetchTool_RuntimeToggle(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{DefuddleEnabled: true})

	// Verify initially has 2 extractors
	tool.mu.RLock()
	if len(tool.chain.extractors) != 2 {
		t.Fatalf("expected 2 extractors initially, got %d", len(tool.chain.extractors))
	}
	tool.mu.RUnlock()

	// Disable defuddle
	tool.UpdateDefuddleEnabled(false)
	tool.mu.RLock()
	if len(tool.chain.extractors) != 1 {
		t.Errorf("expected 1 extractor after disable, got %d", len(tool.chain.extractors))
	}
	tool.mu.RUnlock()

	// Re-enable defuddle
	tool.UpdateDefuddleEnabled(true)
	tool.mu.RLock()
	if len(tool.chain.extractors) != 2 {
		t.Errorf("expected 2 extractors after re-enable, got %d", len(tool.chain.extractors))
	}
	tool.mu.RUnlock()
}

// --- formatFetchResult Tests ---

func TestFormatFetchResult_Basic(t *testing.T) {
	result := formatFetchResult("hello world", "defuddle", "https://example.com", 60000, context.Background())
	if !strings.Contains(result, "URL: https://example.com") {
		t.Error("missing URL in result")
	}
	if !strings.Contains(result, "Extractor: defuddle") {
		t.Error("missing extractor name in result")
	}
	if !strings.Contains(result, "hello world") {
		t.Error("missing content in result")
	}
}

func TestFormatFetchResult_Truncation(t *testing.T) {
	longContent := strings.Repeat("x", 200)
	result := formatFetchResult(longContent, "test", "https://example.com", 100, context.Background())
	if !strings.Contains(result, "Truncated: true") && !strings.Contains(result, "Content-Length:") {
		// Either truncation or temp file path should be present
		if !strings.Contains(result, "Content truncated") {
			t.Error("expected truncation indicator in result")
		}
	}
}
