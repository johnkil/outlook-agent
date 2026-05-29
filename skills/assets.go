package skills

import "embed"

// FS contains the public OpenCode skill files shipped with the binary.
//
//go:embed */SKILL.md
var FS embed.FS
