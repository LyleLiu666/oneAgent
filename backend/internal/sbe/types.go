package sbe

// EditBlock represents a single replacement operation
type EditBlock struct {
	FilePath string
	Search   []string // Lines to find
	Replace  []string // Lines to replace with
}

// MatchResult contains the location of the best match
type MatchResult struct {
	StartLine int     // 0-indexed
	EndLine   int     // Inclusive
	Score     float64 // 0.0 - 1.0, 1.0 is perfect match
}
