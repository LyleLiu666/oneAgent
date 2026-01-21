// Package bocha provides a client for the Bocha AI Search API.
// API Documentation: https://api.bocha.cn/v1
package bocha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://api.bocha.cn/v1"
)

// Freshness options for search queries
const (
	FreshnessNoLimit  = "noLimit"
	FreshnessOneDay   = "oneDay"
	FreshnessOneWeek  = "oneWeek"
	FreshnessOneMonth = "oneMonth"
	FreshnessOneYear  = "oneYear"
)

// SearchRequest represents a search query to the Bocha API.
type SearchRequest struct {
	Query     string `json:"query"`
	Summary   bool   `json:"summary,omitempty"`
	Freshness string `json:"freshness,omitempty"` // noLimit, oneDay, oneWeek, oneMonth, oneYear
	Count     int    `json:"count,omitempty"`     // 1-50, default 10
}

// SearchResponse represents the response from Bocha search API.
type SearchResponse struct {
	Code  int         `json:"code"`
	LogID string      `json:"log_id"`
	Msg   *string     `json:"msg"`
	Data  *SearchData `json:"data"`
}

// SearchData contains the search result data.
type SearchData struct {
	Type         string        `json:"_type"`
	QueryContext *QueryContext `json:"queryContext,omitempty"`
	WebPages     *WebPages     `json:"webPages,omitempty"`
	Images       *Images       `json:"images,omitempty"`
	Videos       *Videos       `json:"videos,omitempty"`
}

// QueryContext contains the original query information.
type QueryContext struct {
	OriginalQuery string `json:"originalQuery"`
}

// WebPages contains web search results.
type WebPages struct {
	WebSearchURL          string    `json:"webSearchUrl"`
	TotalEstimatedMatches int       `json:"totalEstimatedMatches"`
	Value                 []WebPage `json:"value"`
}

// WebPage represents a single web search result.
type WebPage struct {
	ID               *string `json:"id"`
	Name             string  `json:"name"`
	URL              string  `json:"url"`
	DisplayURL       string  `json:"displayUrl"`
	Snippet          string  `json:"snippet"`
	Summary          string  `json:"summary,omitempty"`
	SiteName         string  `json:"siteName"`
	SiteIcon         string  `json:"siteIcon"`
	DatePublished    string  `json:"datePublished,omitempty"`
	DateLastCrawled  string  `json:"dateLastCrawled,omitempty"`
	CachedPageURL    *string `json:"cachedPageUrl"`
	Language         *string `json:"language"`
	IsFamilyFriendly *bool   `json:"isFamilyFriendly"`
	IsNavigational   *bool   `json:"isNavigational"`
}

// Images contains image search results.
type Images struct {
	ID           *string `json:"id"`
	WebSearchURL *string `json:"webSearchUrl"`
	Value        []Image `json:"value"`
}

// Image represents a single image search result.
type Image struct {
	WebSearchURL       *string `json:"webSearchUrl"`
	Name               *string `json:"name"`
	ThumbnailURL       string  `json:"thumbnailUrl"`
	DatePublished      *string `json:"datePublished"`
	ContentURL         string  `json:"contentUrl"`
	HostPageURL        string  `json:"hostPageUrl"`
	ContentSize        *string `json:"contentSize"`
	EncodingFormat     *string `json:"encodingFormat"`
	HostPageDisplayURL *string `json:"hostPageDisplayUrl"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	Thumbnail          *string `json:"thumbnail"`
}

// Videos contains video search results.
type Videos struct {
	ID               *string `json:"id"`
	ReadLink         *string `json:"readLink"`
	WebSearchURL     *string `json:"webSearchUrl"`
	IsFamilyFriendly bool    `json:"isFamilyFriendly"`
	Scenario         string  `json:"scenario"`
	Value            []Video `json:"value"`
}

// Video represents a single video search result.
type Video struct {
	WebSearchURL       string      `json:"webSearchUrl"`
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	ThumbnailURL       string      `json:"thumbnailUrl"`
	Publisher          []Publisher `json:"publisher,omitempty"`
	Creator            *Creator    `json:"creator,omitempty"`
	ContentURL         string      `json:"contentUrl"`
	HostPageURL        string      `json:"hostPageUrl"`
	EncodingFormat     string      `json:"encodingFormat"`
	HostPageDisplayURL string      `json:"hostPageDisplayUrl"`
	Width              int         `json:"width"`
	Height             int         `json:"height"`
	Duration           string      `json:"duration,omitempty"`
	MotionThumbnailURL string      `json:"motionThumbnailUrl,omitempty"`
	EmbedHTML          string      `json:"embedHtml,omitempty"`
	AllowHTTPSEmbed    bool        `json:"allowHttpsEmbed"`
	ViewCount          int         `json:"viewCount,omitempty"`
	Thumbnail          *Thumbnail  `json:"thumbnail,omitempty"`
	AllowMobileEmbed   bool        `json:"allowMobileEmbed"`
	IsSuperfresh       bool        `json:"isSuperfresh"`
	DatePublished      string      `json:"datePublished,omitempty"`
}

// Publisher represents a video publisher.
type Publisher struct {
	Name string `json:"name"`
}

// Creator represents a video creator.
type Creator struct {
	Name string `json:"name"`
}

// Thumbnail represents a video thumbnail dimensions.
type Thumbnail struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

// Client is a Bocha API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Bocha API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs a web search using the Bocha API.
func (c *Client) Search(req SearchRequest) (*SearchResponse, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Count == 0 {
		req.Count = 10
	}
	if req.Freshness == "" {
		req.Freshness = FreshnessNoLimit
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/web-search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &searchResp, nil
}

// Search is a convenience function that creates a client and performs a search.
func Search(apiKey string, req SearchRequest) (*SearchResponse, error) {
	client := NewClient(apiKey)
	return client.Search(req)
}
