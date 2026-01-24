package tool

import "github.com/liu_y/oneAgent/backend/internal/llm"

// multiedit is kept as an alias for edit, to reduce model confusion and preserve compatibility.
// It shares the same handler as edit and accepts the same payload shape: {edits: [...], replaceAll?: bool}.
func multiEditDefinition() Definition {
	spec := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "multiedit",
			Description: "批量对已有文件做“模糊替换”(fuzzy patch)，等同于 edit 的 edits 参数（别名工具，用于兼容/降低心智负担）。强烈建议分段、小步、多次调用：单次 oldString/newString 建议 ≤3000 字、单次 edits ≤10、总字数建议 ≤12000。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"edits": map[string]any{
						"type":        "array",
						"description": "要应用的编辑列表（建议每次只改 1-3 处，小步迭代）。",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"filePath": map[string]any{
									"type":        "string",
									"description": "要编辑的文件路径（相对沙箱根目录，或沙箱根目录内的绝对路径）。",
								},
								"oldString": map[string]any{
									"type":        "string",
									"description": "要搜索/匹配的原始片段（需要足够独特以定位；建议长度适中）。",
								},
								"newString": map[string]any{
									"type":        "string",
									"description": "替换后的片段（建议分段提交）。",
								},
								"replaceAll": map[string]any{
									"type":        "boolean",
									"description": "是否替换所有匹配（true=全部替换；false=仅替换第一个/最佳匹配）。",
								},
							},
							"required":             []string{"filePath", "oldString", "newString"},
							"additionalProperties": false,
						},
					},
					"replaceAll": map[string]any{
						"type":        "boolean",
						"description": "（可选）对本次 edits 全局生效的 replaceAll；true 时会覆盖每个 edit 的 replaceAll。",
					},
				},
				"required":             []string{"edits"},
				"additionalProperties": false,
			},
		},
	}

	return newDefinition(ToolIDMultiEdit, spec, runSmartEditTool)
}
