package tool

import "strings"

type SafetyMetadata struct {
	Effect        string `json:"effect"`
	Reversibility string `json:"reversibility"`
}

const (
	SafetyEffectReadOnly = "read_only"
	SafetyEffectMutating = "mutating"

	SafetyReversibilityRollbackable = "rollbackable"
	SafetyReversibilityIrreversible = "irreversible"
)

func SafetyForToolID(toolID string) SafetyMetadata {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return SafetyMetadata{
			Effect:        SafetyEffectMutating,
			Reversibility: SafetyReversibilityIrreversible,
		}
	}

	switch toolID {
	case ToolIDReadFile,
		ToolIDLs,
		ToolIDGlob,
		ToolIDRg,
		ToolIDSearch,
		ToolIDSkillRead,
		ToolIDLSPDefinition,
		ToolIDLSPReferences,
		ToolIDLSPRenamePreview:
		return SafetyMetadata{
			Effect:        SafetyEffectReadOnly,
			Reversibility: SafetyReversibilityRollbackable,
		}
	case ToolIDWriteFile,
		ToolIDEdit,
		ToolIDEditV2,
		ToolIDMultiEdit,
		ToolIDTrashFile,
		ToolIDDocumentExport,
		ToolIDPlan:
		return SafetyMetadata{
			Effect:        SafetyEffectMutating,
			Reversibility: SafetyReversibilityRollbackable,
		}
	case ToolIDBash,
		ToolIDRunCommand,
		ToolIDSubagent:
		return SafetyMetadata{
			Effect:        SafetyEffectMutating,
			Reversibility: SafetyReversibilityIrreversible,
		}
	default:
		// Conservative default for unknown/custom tools.
		return SafetyMetadata{
			Effect:        SafetyEffectMutating,
			Reversibility: SafetyReversibilityIrreversible,
		}
	}
}
