# ToolXML 模块：设计

> `backend/internal/toolxml`

> 本文档包含原技术规格的第 3 章（流程与可视化）；其余章节见 [细节](01-details.md)。

---

## 3. Logic Flow & Visualization

### 3.1 RunLoop 执行流程

```mermaid
graph TD
    A[Start] --> B[获取 LLM 响应]
    B --> C[streamFilter + 回调]
    C --> D[StripThinking]
    D --> E{ExtractLatestToolData?}
    E -->|No| F[observeFinal + 返回]
    E -->|Yes| G[ParseToolData]
    G --> H{每个 Call}
    H --> I[buildToolArgs]
    I --> J[执行 Handler]
    J --> K[收集 ToolResult]
    K --> L[buildToolResultMessage]
    L --> M[observeStep]
    M --> N[追加到 messages]
    N --> O{step < 20?}
    O -->|Yes| B
    O -->|No| P[返回 limit 错误]
```

### 3.2 StreamFilter 状态机

```mermaid
stateDiagram-v2
    [*] --> Normal
    Normal --> InToolData: <tool_data>
    Normal --> InThinking: <thinking> 或 <think>
    InToolData --> Normal: </tool_data>
    InThinking --> Normal: </thinking> 或 </think>
    
    note right of InToolData
        抑制输出到用户
        保留尾部 64 字符
    end note
```

### 3.3 Parser 流程

```go
// 1. 提取最后一个 <tool_data>
block, ok := ExtractLatestToolData(rawResponse)

// 2. 解析所有 <call> 块
calls, err := ParseToolData(block)

// 3. 每个 Call 包含:
//    - ToolName (从 tool_name/tool/name 提取)
//    - Fields (支持 CDATA, HTML unescape)
//    - Raw (原始 XML 用于调试)
```

