# SBE 模块：设计

> `backend/internal/sbe`

> 本文档包含原技术规格的第 2/3 章（算法与流程）；其余章节见 [细节](01-details.md)。

---

## 2. 9 层瀑布匹配算法

### 2.1 算法流程

```mermaid
graph TD
    A[输入 sourceLines, searchLines] --> B{1. Exact Match}
    B -->|Found| Z1[Score=1.00]
    B -->|Not Found| C{2. Trimmed Match}
    C -->|Found| Z2[Score=0.95]
    C -->|Not Found| D{3. Anchor Match}
    D -->|Found| Z3[Score=similarity]
    D -->|Not Found| E{4. Whitespace Normalized}
    E -->|Found| Z4[Score=0.90]
    E -->|Not Found| F{5. Indentation Flexible}
    F -->|Found| Z5[Score=0.85]
    F -->|Not Found| G{6. Escape Normalized}
    G -->|Found| Z6[Score=0.80]
    G -->|Not Found| H{7. Trimmed Boundary}
    H -->|Found| Z7[Score=0.75]
    H -->|Not Found| I{8. Context Aware}
    I -->|Found| Z8[Score=0.70]
    I -->|Not Found| J[返回 nil]
```

### 2.2 策略详解

| 层级 | 策略名称 | 匹配逻辑 | 分数 |
|-----|---------|---------|------|
| **1** | Exact Match | 逐行精确字符串比较 | 1.00 |
| **2** | Trimmed Match | `TrimSpace()` 后逐行比较 | 0.95 |
| **3** | Anchor Match | 首尾行精确 + 中间 Levenshtein ≥0.6 | 0.60-0.99 |
| **4** | Whitespace Normalized | 所有空白合并为单空格后比较 | 0.90 |
| **5** | Indentation Flexible | 移除公共缩进后比较 | 0.85 |
| **6** | Escape Normalized | 处理 `\n`, `\t`, `\'`, `\"` 等转义 | 0.80 |
| **7** | Trimmed Boundary | 移除首尾空行后用 Trimmed Match | 0.75 |
| **8** | Context Aware | 首尾锚定 + 中间存在即可 | 0.70 |
| **9** | Multi Occurrence | (预留，当前返回 nil) | - |

### 2.3 Levenshtein 距离

```go
// ComputeDistance 计算两字符串的编辑距离
func ComputeDistance(a, b string) int

// 相似度计算
similarity := 1.0 - (float64(distance) / float64(max(len(a), len(b))))
```

**Anchor Match 阈值**: `SimilarityThreshold = 0.6`

---

## 3. Logic Flow & Visualization

### 3.1 ApplyEditBlocks 流程

```mermaid
sequenceDiagram
    participant C as Caller
    participant A as Actuator
    participant M as Matcher
    participant FS as FileSystem

    C->>A: ApplyEditBlocks(blocks)
    loop 每个 block
        A->>FS: ReadFile(filePath)
        FS-->>A: sourceLines
        
        alt ReplaceAll
            A->>M: FindBestMatch (循环)
            M-->>A: MatchResult[]
            A->>A: 依次替换
        else Single
            A->>M: FindBestMatch
            M-->>A: MatchResult
            A->>A: applyReplacement
        end
        
        A->>FS: WriteFile(updated)
    end
    A-->>C: (totalReplacements, error)
```

### 3.2 Anchor Match 伪码

```python
function findAnchorMatch(source, search):
    if len(search) < 3:
        return nil
    
    firstLine = TrimSpace(search[0])
    lastLine = TrimSpace(search[-1])
    
    for i in range(len(source)):
        if TrimSpace(source[i]) != firstLine:
            continue
        
        maxDist = len(search) * 2
        for j in range(i+1, min(len(source), i+maxDist)):
            if TrimSpace(source[j]) == lastLine:
                sourceBlock = normalize(join(source[i:j+1]))
                searchBlock = normalize(join(search))
                
                distance = ComputeDistance(sourceBlock, searchBlock)
                similarity = 1.0 - (distance / max(len(sourceBlock), len(searchBlock)))
                
                if similarity >= 0.6:
                    return MatchResult(i, j, similarity)
    
    return nil
```

