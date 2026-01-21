package llm

import "sort"

func normalizeTools(tools []Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}

	ordered := make([]Tool, len(tools))
	copy(ordered, tools)

	sort.SliceStable(ordered, func(i, j int) bool {
		leftID := ordered[i].ID
		rightID := ordered[j].ID
		if leftID == "" || rightID == "" {
			return ordered[i].Function.Name < ordered[j].Function.Name
		}
		if leftID == rightID {
			return ordered[i].Function.Name < ordered[j].Function.Name
		}
		return leftID < rightID
	})

	return ordered
}
