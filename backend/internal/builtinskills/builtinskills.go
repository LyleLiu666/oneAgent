package builtinskills

import "embed"

// FS contains built-in skills packaged into the oneagent binary.
//
// Layout:
//
//	skills/<skill-id>/SKILL.md
//
// NOTE: The skill system only requires SKILL.md today, but we embed the full
// skills directory so future skills may ship with scripts/references/assets.
//
//go:embed skills
var FS embed.FS
