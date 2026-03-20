package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InProcessExtractor does a direct HTTP GET and converts HTML to markdown
// using the built-in DOM-based converter. This is the fallback when the
// Defuddle CF Worker is unavailable or returns low-quality content.
type InProcessExtractor struct {
	client *http.Client
}

// NewInProcessExtractor creates an InProcessExtractor with a reusable HTTP client.
func NewInProcessExtractor() *InProcessExtractor {
	return &InProcessExtractor{
		client: &http.Client{
			Timeout: time.Duration(fetchTimeoutSeconds) * time.Second,
			Transport: &http.Transport{
				ForceAttemptHTTP2:   true,
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 15 * time.Second,
			},
		},
	}
}

func (e *InProcessExtractor) Name() string { return "html-to-markdown" }

// Extract fetches the URL directly and converts the response to markdown.
// Handles HTML, JSON, markdown, and raw text content types.
func (e *InProcessExtractor) Extract(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	redirectCount := 0
	// Use a per-request copy of CheckRedirect since it captures mutable state.
	client := *e.client
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		redirectCount++
		if redirectCount > defaultFetchMaxRedirect {
			return fmt.Errorf("stopped after %d redirects", defaultFetchMaxRedirect)
		}
		if err := CheckSSRF(r.URL.String()); err != nil {
			return fmt.Errorf("redirect SSRF protection: %w", err)
		}
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	readLimit := int64(max(defaultFetchMaxChars*10, 512*1024))
	body, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")

	switch {
	case strings.Contains(contentType, "application/json"):
		text, _ := extractJSON(body)
		return text, nil

	case strings.Contains(contentType, "text/markdown"):
		return string(body), nil

	case strings.Contains(contentType, "text/html"),
		strings.Contains(contentType, "application/xhtml"):
		text := htmlToMarkdown(string(body))
		if text == "" && len(body) > 0 {
			return "", fmt.Errorf("no content extracted from HTML (page may require JavaScript)")
		}
		return text, nil

	default:
		return string(body), nil
	}
}
