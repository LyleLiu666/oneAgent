package toolxml

import (
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/tool"
)

var supportedToolNames = map[string]struct{}{
	"bash":        {},
	"edit":        {},
	"glob":        {},
	"ls":          {},
	"multiedit":   {},
	"plan":        {},
	"read_file":   {},
	"rg":          {},
	"run_command": {},
	"search":      {},
	"skill_read":  {},
	"subagent":    {},
	"write_file":  {},
}

func IsSupportedToolName(name string) bool {
	canonical := tool.CanonicalToolName(strings.TrimSpace(name))
	if canonical == "" {
		return false
	}
	_, ok := supportedToolNames[canonical]
	return ok
}

// FilterSupportedDefinitions keeps only tool definitions supported by the XML engine.
// It returns the filtered list and a list of dropped tool function names.
func FilterSupportedDefinitions(defs []tool.Definition) ([]tool.Definition, []string) {
	if len(defs) == 0 {
		return defs, nil
	}

	supported := make([]tool.Definition, 0, len(defs))
	dropped := make([]string, 0)
	for _, def := range defs {
		name := strings.TrimSpace(def.Spec.Function.Name)
		if IsSupportedToolName(name) {
			supported = append(supported, def)
			continue
		}
		if name != "" {
			dropped = append(dropped, name)
		} else if strings.TrimSpace(def.ID) != "" {
			dropped = append(dropped, def.ID)
		} else {
			dropped = append(dropped, "<unknown>")
		}
	}
	return supported, dropped
}
