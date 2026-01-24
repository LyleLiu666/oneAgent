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
			Description: "联网搜索（通过 Bocha 搜索引擎）。适用于查最新信息、检索资料、核对事实。返回包含标题/URL/摘要的结果列表。建议 query 简洁明确，必要时分多次搜索。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词（尽量具体，包含关键实体/时间/地点）。",
					},
					"count": map[string]any{
						"type":        "integer",
						"description": "返回结果数量（1-50），默认 10。",
						"minimum":     1,
						"maximum":     50,
					},
					"freshness": map[string]any{
						"type":        "string",
						"description": "时间筛选：noLimit(默认)/oneDay/oneWeek/oneMonth/oneYear。",
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
		return nil, fmt.Errorf("解析 search 参数失败: %w", err)
	}

	if args.Query == "" {
		return nil, fmt.Errorf("query 不能为空")
	}

	// Get userID from context
	userID := UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("缺少用户上下文")
	}

	// Get API key from database settings
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("数据库不可用")
	}

	apiKey, err := model.GetUserSetting(db, userID, model.SettingKeyBochaAPIKey)
	if err != nil || apiKey == "" {
		return nil, fmt.Errorf("未配置 Bocha API Key，请在 Settings 中设置")
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
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	if resp.Code != 200 && resp.Msg != nil {
		return nil, fmt.Errorf("搜索接口返回错误: %s", *resp.Msg)
	}

	// Format results
	return formatSearchResults(resp), nil
}

func formatSearchResults(resp *bocha.SearchResponse) string {
	if resp == nil || resp.Data == nil || resp.Data.WebPages == nil || len(resp.Data.WebPages.Value) == 0 {
		return "未找到搜索结果。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共找到 %d 条结果：\n\n", len(resp.Data.WebPages.Value)))

	for i, page := range resp.Data.WebPages.Value {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, page.Name))
		sb.WriteString(fmt.Sprintf("**URL：** %s\n", page.URL))
		if page.SiteName != "" {
			sb.WriteString(fmt.Sprintf("**站点：** %s\n", page.SiteName))
		}
		if page.DatePublished != "" {
			sb.WriteString(fmt.Sprintf("**发布时间：** %s\n", page.DatePublished))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n", page.Snippet))
		if page.Summary != "" {
			sb.WriteString(fmt.Sprintf("\n**摘要：** %s\n", page.Summary))
		}
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}
