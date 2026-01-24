# Bocha 模块：细节

> `backend/internal/bocha`

> 本文档包含原技术规格的第 1/3/4 章；第 2 章（流程与可视化）见 [设计](02-design.md)。

---

## 1. API & Interface Contract

### 客户端接口

```go
// 创建客户端
func NewClient(apiKey string) *Client

// 执行搜索
func (c *Client) Search(req SearchRequest) (*SearchResponse, error)
```

### SearchRequest

```json
{
    "query": "String (Required)",
    "summary": "Boolean (Optional, 返回摘要)",
    "freshness": "Enum[noLimit, oneDay, oneWeek, oneMonth, oneYear]",
    "count": "Integer (1-50, Default: 10)"
}
```

### SearchResponse

```json
{
    "code": "Integer",
    "log_id": "String",
    "msg": "String (nullable)",
    "data": {
        "_type": "String",
        "queryContext": { "originalQuery": "String" },
        "webPages": {
            "webSearchUrl": "String",
            "totalEstimatedMatches": "Integer",
            "value": [WebPage]
        },
        "images": { "value": [Image] },
        "videos": { "value": [Video] }
    }
}
```

### Handler API

```
GET /api/bocha/settings
Response: { "has_bocha_api_key": Boolean }

PUT /api/bocha/settings
Request: { "api_key": "String" }
Response: { "message": "saved" }

POST /api/bocha/search
Request: { "query": "...", "count": 10, "freshness": "oneWeek" }
Response: SearchResponse
```

---

## 3. Sad Path Matrix

| 场景 | 异常类型 | 处理策略 | 客户端表现 |
|------|---------|---------|-----------|
| **API Key 未配置** | Config | 返回 "API key not configured" | 提示配置 |
| **空查询** | Validation | 返回 "query is required" | 参数错误 |
| **API 返回非 200** | Upstream | 返回 API 错误信息 | 显示搜索失败 |
| **网络超时** | Network | 30s 超时返回错误 | 显示超时 |
| **响应解析失败** | Parse | 返回 unmarshal 错误 | 搜索失败 |

---

## 4. Data Persistence

API Key 存储在 `user_settings` 表：

```sql
INSERT INTO user_settings (user_id, key, value) 
VALUES ('user123', 'bocha_api_key', 'sk-xxx')
ON CONFLICT (user_id, key) DO UPDATE SET value = 'sk-xxx';
```

### Tool 集成

作为 `search` 工具注册：

```go
type searchToolRequest struct {
    Query     string `json:"query"`
    Count     int    `json:"count,omitempty"`
    Freshness string `json:"freshness,omitempty"`
}
```

