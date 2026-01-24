# SBE 模块：细节

> `backend/internal/sbe`

> 本文档包含原技术规格的第 1/4/5 章；第 2/3 章（算法与流程）见 [设计](02-design.md)。

---

## 1. API & Interface Contract

### 1.1 核心接口

```go
// FindBestMatch 在源文件中查找搜索块的最佳匹配位置
func FindBestMatch(sourceLines []string, searchLines []string) *MatchResult

// ApplyEditBlocks 应用编辑块列表，返回总替换次数
func ApplyEditBlocks(blocks []EditBlock) (int, error)
```

### 1.2 数据类型

```go
type MatchResult struct {
    StartLine int     // 匹配起始行 (0-indexed)
    EndLine   int     // 匹配结束行 (inclusive)
    Score     float64 // 匹配分数 (0.0-1.0)
}

type EditBlock struct {
    FilePath   string   // 目标文件路径
    Search     []string // 搜索行
    Replace    []string // 替换行
    ReplaceAll bool     // 是否替换所有匹配
}
```

---

## 4. Sad Path Matrix

| 场景 | 函数 | 错误消息 | 处理 |
|------|------|---------|------|
| **空搜索块** | FindBestMatch | 返回 nil | 调用方处理 |
| **文件路径为空** | applyBlock | `file path is required` | 拒绝 |
| **搜索块为空** | applyBlock | `search block is empty for {file}` | 拒绝 |
| **文件读取失败** | applyBlock | `read file error: {err}` | 拒绝 |
| **无匹配** | applyBlock | `could not find search block in {file}` | 拒绝 |
| **文件写入失败** | writeLines | `write file error: {err}` | 拒绝 |
| **相似度过低** | findAnchorMatch | 返回 nil | 尝试下一层 |

---

## 5. Data Persistence

SBE 是纯内存模块，不涉及持久化。

### 5.1 辅助函数

```go
// normalize 移除所有空白用于松散比较
func normalize(s string) string {
    return strings.Join(strings.Fields(s), "")
}
```

### 5.2 转义处理

```go
// unescape 处理常见转义序列
func unescape(input string) string {
    // \n → 换行
    // \t → 制表符
    // \r → 回车
    // \' → 单引号
    // \" → 双引号
    // \` → 反引号
    // \$ → 美元符
    // \\ → 反斜杠
}
```

### 5.3 缩进剥离

```go
// stripIndent 移除所有行的公共缩进前缀
func stripIndent(lines []string) string {
    // 1. 找到非空行的最小缩进
    // 2. 从每行移除该缩进长度
    // 3. 拼接返回
}
```

