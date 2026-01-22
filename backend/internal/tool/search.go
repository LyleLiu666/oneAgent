package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/bocha"
	"github.com/liu_y/oneAgent/backend/internal/database"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

type searchArgs struct {
	Query     string `json:"query"`
	Count     int    `json:"count,omitempty"`
	Freshness string `json:"freshness,omitempty"`
}

func searchDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "search",
			Description: "Search the web for information using a search engine. Returns web pages with titles, URLs, and snippets. Use this tool when you need to find current information, research topics, or look up facts that may not be in your training data.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query string. Be specific and use relevant keywords for better results.",
					},
					"count": map[string]any{
						"type":        "integer",
						"description": "Number of results to return (1-50). Default is 10.",
						"minimum":     1,
						"maximum":     50,
					},
					"freshness": map[string]any{
						"type":        "string",
						"description": "Time filter for results. Options: 'noLimit' (default), 'oneDay', 'oneWeek', 'oneMonth', 'oneYear'.",
						"enum":        []string{"noLimit", "oneDay", "oneWeek", "oneMonth", "oneYear"},
					},
				},
				"required":             []string{"query"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDSearch, spec, searchHandler)
}

func searchHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var args searchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to parse search arguments: %w", err)
	}

	if args.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Get userID from context
	userID := UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user context not available")
	}

	// Get API key from database settings
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	apiKey, err := model.GetUserSetting(db, userID, model.SettingKeyBochaAPIKey)
	if err != nil || apiKey == "" {
		return nil, fmt.Errorf("Bocha API key not configured. Please set it in Settings.")
	}

	// Set defaults
	count := args.Count
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}

	freshness := args.Freshness
	if freshness == "" {
		freshness = bocha.FreshnessNoLimit
	}

	// Perform search
	req := bocha.SearchRequest{
		Query:     args.Query,
		Count:     count,
		Freshness: freshness,
		Summary:   true,
	}

	resp, err := bocha.Search(apiKey, req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if resp.Code != 200 && resp.Msg != nil {
		return nil, fmt.Errorf("search API error: %s", *resp.Msg)
	}

	// Format results
	return formatSearchResults(resp), nil
}

func formatSearchResults(resp *bocha.SearchResponse) string {
	if resp == nil || resp.Data == nil || resp.Data.WebPages == nil || len(resp.Data.WebPages.Value) == 0 {
		return "No search results found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(resp.Data.WebPages.Value)))

	for i, page := range resp.Data.WebPages.Value {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, page.Name))
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", page.URL))
		if page.SiteName != "" {
			sb.WriteString(fmt.Sprintf("**Site:** %s\n", page.SiteName))
		}
		if page.DatePublished != "" {
			sb.WriteString(fmt.Sprintf("**Published:** %s\n", page.DatePublished))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n", page.Snippet))
		if page.Summary != "" {
			sb.WriteString(fmt.Sprintf("\n**Summary:** %s\n", page.Summary))
		}
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}
