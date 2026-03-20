package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// ContentExtractor extracts readable content from a URL.
type ContentExtractor interface {
	Extract(ctx context.Context, rawURL string) (string, error)
	Name() string
}

// ExtractResult holds the output from a successful extraction.
type ExtractResult struct {
	Content   string
	Extractor string // name of the extractor that succeeded
}

// ExtractorChain tries extractors in order until one returns quality content.
type ExtractorChain struct {
	extractors []ContentExtractor
}

// NewExtractorChain creates a chain from ordered extractors.
func NewExtractorChain(extractors ...ContentExtractor) *ExtractorChain {
	return &ExtractorChain{extractors: extractors}
}

// Extract runs each extractor in order, returning the first quality result.
func (c *ExtractorChain) Extract(ctx context.Context, rawURL string) (ExtractResult, error) {
	var lastErr error
	for _, ext := range c.extractors {
		content, err := ext.Extract(ctx, rawURL)
		if err != nil {
			slog.Debug("extractor failed", "extractor", ext.Name(), "url", rawURL, "error", err)
			lastErr = err
			continue
		}
		if !isQualityContent(content) {
			slog.Debug("extractor returned low quality content", "extractor", ext.Name(), "url", rawURL, "chars", len(content))
			lastErr = fmt.Errorf("%s: content below quality threshold (%d chars)", ext.Name(), len(content))
			continue
		}
		return ExtractResult{Content: content, Extractor: ext.Name()}, nil
	}
	if lastErr != nil {
		return ExtractResult{}, fmt.Errorf("all extractors failed for %s: %w", rawURL, lastErr)
	}
	return ExtractResult{}, fmt.Errorf("no extractors configured")
}

// isQualityContent checks if extracted content meets minimum quality thresholds.
// Returns false for empty, very short (<100 chars), or low word count (<10 words) content.
func isQualityContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < 100 {
		return false
	}
	return len(strings.Fields(trimmed)) >= 10
}
