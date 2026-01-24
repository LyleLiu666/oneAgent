# Handler 模块：设计

> `backend/internal/handler`

> 本文档包含原技术规格的第 2 章（流程与可视化）；其余章节见 [细节](01-details.md)。

---

## 2. Logic Flow & Visualization

### 2.1 StreamChat 完整处理流程

```mermaid
sequenceDiagram
    participant C as Client
    participant H as ChatHandler
    participant SM as StreamManager
    participant DB as Database
    participant LLM as LLM Provider

    C->>H: POST /api/chat
    H->>H: 解析请求参数
    H->>DB: 获取/创建 Session
    H->>DB: 加载历史消息
    H->>H: 构建 LLM messages[]
    H->>H: 解析工具定义 (tool.Mount)
    H->>H: 解析模型配置 (resolveModel)
    
    alt ToolProtocol == "xml"
        H->>H: 注入 ToolXML System Prompt
    end
    
    H->>SM: GetOrCreate(sessionID)
    H->>SM: Subscribe()
    H-->>C: SSE: session_id
    
    alt 首个客户端 (StartGeneration)
        H->>H: compressSessionIfNeeded
        H->>DB: 保存用户消息
        
        alt 无工具 / JSON 工具
            H->>LLM: ChatCompletionStream
            loop 每个 token
                LLM-->>SM: Broadcast(delta)
                SM-->>C: SSE: msg
            end
        else XML 工具协议
            H->>H: toolxml.RunLoop (最多 20 步)
            loop 每步
                H->>LLM: 请求
                LLM-->>H: 响应 + tool_data
                H->>H: 解析 XML 工具调用
                H->>H: 执行工具
                H-->>SM: Broadcast(tool_call, tool_result)
                H->>DB: 保存 tool_call/tool_result
            end
        end
        
        H->>DB: 保存助手消息
        H->>SM: Finish()
    end
    
    H-->>C: SSE: done
```

### 2.2 会话压缩流程

```mermaid
graph TD
    A[检查上下文长度] --> B{runes > 80000?}
    B -->|No| C[不压缩]
    B -->|Yes| D[分割消息]
    D --> E[保留最近 4 条文本消息]
    D --> F[格式化历史为摘要输入]
    F --> G[调用 LLM 生成摘要]
    G --> H{成功?}
    H -->|Yes| I[删除旧消息]
    I --> J[插入摘要消息]
    J --> K[重建 LLM messages]
    H -->|No| L[降级: 仅保留最近 2 轮]
```

**压缩常量**:

| 常量 | 值 | 说明 |
|------|---|------|
| `sessionCompressionMaxContextRunes` | 80,000 | 触发压缩阈值 |
| `sessionCompressionKeepTextMsgs` | 4 | 保留最近消息数 |
| `sessionCompressionMaxSummaryInputRunes` | 120,000 | 摘要输入上限 |
| `sessionCompressionMaxMsgRunesForInput` | 4,000 | 单消息截断长度 |

### 2.3 StreamBroadcaster 机制

```mermaid
graph LR
    subgraph Session
        G[Generator Goroutine]
        B[StreamBroadcaster]
    end
    
    G -->|Broadcast| B
    B -->|Fan-out| C1[Client 1 Channel]
    B -->|Fan-out| C2[Client 2 Channel]
    B -->|Fan-out| C3[Client N Channel]
    
    C1 --> SSE1[SSE Response 1]
    C2 --> SSE2[SSE Response 2]
    C3 --> SSE3[SSE Response N]
```

**关键方法**:
- `GetOrCreate(sessionID)`: 获取或创建会话广播器
- `Subscribe()`: 订阅事件通道
- `Unsubscribe(ch)`: 取消订阅
- `StartGeneration()`: 原子标记生成开始 (仅首个返回 true)
- `Broadcast(event)`: 向所有订阅者发送事件
- `Finish()`: 关闭所有订阅通道

