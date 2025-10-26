package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/crush/internal/permission"
)

type WebSearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type webSearchTool struct {
	client      *http.Client
	permissions permission.Service
	workingDir  string
}

const WebSearchToolName = "websearch"

//go:embed websearch.md
var webSearchDescription []byte

func NewWebSearchTool(permissions permission.Service, workingDir string) BaseTool {
	return &webSearchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *webSearchTool) Name() string {
	return WebSearchToolName
}

func (t *webSearchTool) Info() ToolInfo {
	return ToolInfo{
		Name:        WebSearchToolName,
		Description: string(webSearchDescription),
		Parameters: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of results to return (default: 10, max: 20)",
			},
		},
		Required: []string{"query"},
	}
}

func (t *webSearchTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params WebSearchParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("Failed to parse search parameters: " + err.Error()), nil
	}

	if params.Query == "" {
		return NewTextErrorResponse("Query parameter is required"), nil
	}

	// Default to 10 results, max 20
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 20 {
		maxResults = 20
	}

	// No permission needed for web search - it's read-only and safe
	results, err := t.searchDuckDuckGo(ctx, params.Query, maxResults)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return NewTextResponse("No results found for query: " + params.Query), nil
	}

	// Format results as text
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for \"%s\":\n\n", params.Query))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Snippet != "" {
			output.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
		output.WriteString("\n")
	}

	return NewTextResponse(output.String()), nil
}

func (t *webSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// Use DuckDuckGo HTML search
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult

	// Extract search results from DuckDuckGo HTML
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= maxResults {
			return
		}

		// Get title and URL
		titleElem := s.Find(".result__a")
		title := strings.TrimSpace(titleElem.Text())
		href, exists := titleElem.Attr("href")

		if !exists || title == "" {
			return
		}

		// DuckDuckGo uses redirect URLs, extract the actual URL
		actualURL := extractURLFromDDGRedirect(href)

		// Get snippet
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		results = append(results, SearchResult{
			Title:   title,
			URL:     actualURL,
			Snippet: snippet,
		})
	})

	return results, nil
}

func extractURLFromDDGRedirect(ddgURL string) string {
	// DuckDuckGo uses URLs like: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com
	if strings.Contains(ddgURL, "uddg=") {
		parts := strings.Split(ddgURL, "uddg=")
		if len(parts) > 1 {
			decoded, err := url.QueryUnescape(parts[1])
			if err == nil {
				return decoded
			}
		}
	}
	return ddgURL
}
