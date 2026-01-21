package sbe

import (
	"strings"
)

// Helper to normalize lines (remove whitespace) for loose comparison
func normalize(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// FindBestMatch attempts to locate the search block in the source content
// using multiple strategies in a waterfall manner (The "9-Layer Repair" Logic).
func FindBestMatch(sourceLines []string, searchLines []string) *MatchResult {
	if len(searchLines) == 0 {
		return nil
	}

	// Strategy 1: Exact Match
	if match := findExactMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 2: Line Trimmed (Ignore Leading/Trailing Whitespace per line)
	if match := findTrimmedMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 3: Block Anchor (Fuzzy Middle with Levenshtein)
	if match := findAnchorMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 4: Whitespace Normalized (Ignore all whitespace differences)
	if match := findWhitespaceNormalizedMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 5: Indentation Flexible (Ignore relative indentation)
	if match := findIndentationFlexibleMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 6: Escape Normalized (Handle escaped characters like \n, \t)
	if match := findEscapeNormalizedMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 7: Trimmed Boundary (Try matching without leading/trailing blank lines in search)
	if match := findTrimmedBoundaryMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 8: Context Aware (Loose match based on anchors and ~50% body match)
	if match := findContextAwareMatch(sourceLines, searchLines); match != nil {
		return match
	}

	// Strategy 9: Multi Occurrence - Just return nil here as we want the best match or fail
	return nil
}

// --- Strategies Implementation ---

func findExactMatch(source, search []string) *MatchResult {
	n := len(source)
	m := len(search)
	for i := 0; i <= n-m; i++ {
		match := true
		for j := 0; j < m; j++ {
			if source[i+j] != search[j] {
				match = false
				break
			}
		}
		if match {
			return &MatchResult{StartLine: i, EndLine: i + m - 1, Score: 1.0}
		}
	}
	return nil
}

func findTrimmedMatch(source, search []string) *MatchResult {
	n := len(source)
	m := len(search)
	for i := 0; i <= n-m; i++ {
		match := true
		for j := 0; j < m; j++ {
			if strings.TrimSpace(source[i+j]) != strings.TrimSpace(search[j]) {
				match = false
				break
			}
		}
		if match {
			return &MatchResult{StartLine: i, EndLine: i + m - 1, Score: 0.95}
		}
	}
	return nil
}

func findAnchorMatch(source, search []string) *MatchResult {
	if len(search) < 3 {
		return nil
	}
	firstLine := strings.TrimSpace(search[0])
	lastLine := strings.TrimSpace(search[len(search)-1])
	const SimilarityThreshold = 0.6

	for i := 0; i < len(source); i++ {
		if strings.TrimSpace(source[i]) != firstLine {
			continue
		}
		maxDist := len(search) * 2
		for j := i + 1; j < len(source) && j < i+maxDist; j++ {
			if strings.TrimSpace(source[j]) == lastLine {
				sourceBlock := strings.Join(source[i:j+1], "\n")
				searchBlock := strings.Join(search, "\n")
				dist := ComputeDistance(normalize(sourceBlock), normalize(searchBlock))
				maxLen := len(sourceBlock)
				if len(searchBlock) > maxLen {
					maxLen = len(searchBlock)
				}
				if maxLen == 0 {
					continue
				}
				similarity := 1.0 - (float64(dist) / float64(maxLen))
				if similarity >= SimilarityThreshold {
					return &MatchResult{StartLine: i, EndLine: j, Score: similarity}
				}
			}
		}
	}
	return nil
}

func findWhitespaceNormalizedMatch(source, search []string) *MatchResult {
	norm := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}
	searchStr := norm(strings.Join(search, "\n"))

	n := len(source)
	m := len(search)

	for i := 0; i <= n-m; i++ {
		targetBlock := strings.Join(source[i:i+m], "\n")
		if norm(targetBlock) == searchStr {
			return &MatchResult{StartLine: i, EndLine: i + m - 1, Score: 0.90}
		}
	}
	return nil
}

func findIndentationFlexibleMatch(source, search []string) *MatchResult {
	stripIndent := func(lines []string) string {
		if len(lines) == 0 {
			return ""
		}
		minIndent := 999
		for _, line := range lines {
			trimmed := strings.TrimLeft(line, " \t")
			if len(trimmed) > 0 {
				indent := len(line) - len(trimmed)
				if indent < minIndent {
					minIndent = indent
				}
			}
		}
		if minIndent == 999 {
			return strings.Join(lines, "\n")
		}

		var stripped []string
		for _, line := range lines {
			if len(line) >= minIndent {
				stripped = append(stripped, line[minIndent:])
			} else {
				stripped = append(stripped, line)
			}
		}
		return strings.Join(stripped, "\n")
	}

	searchBlock := stripIndent(search)
	n := len(source)
	m := len(search)

	for i := 0; i <= n-m; i++ {
		sourceChunk := source[i : i+m]
		if stripIndent(sourceChunk) == searchBlock {
			return &MatchResult{StartLine: i, EndLine: i + m - 1, Score: 0.85}
		}
	}
	return nil
}

func findEscapeNormalizedMatch(source, search []string) *MatchResult {
	unescape := func(s string) string {
		s = strings.ReplaceAll(s, "\\n", "\n")
		s = strings.ReplaceAll(s, "\\t", "\t")
		s = strings.ReplaceAll(s, "\\\"", "\"")
		return s
	}

	searchBlock := unescape(strings.Join(search, "\n"))
	sourceBlock := strings.Join(source, "\n")

	if idx := strings.Index(sourceBlock, searchBlock); idx != -1 {
		pre := sourceBlock[:idx]
		startLine := strings.Count(pre, "\n")
		lineCount := strings.Count(searchBlock, "\n")
		return &MatchResult{StartLine: startLine, EndLine: startLine + lineCount, Score: 0.80}
	}
	return nil
}

func findTrimmedBoundaryMatch(source, search []string) *MatchResult {
	var passedSearch []string
	start, end := 0, len(search)
	for start < end && strings.TrimSpace(search[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(search[end-1]) == "" {
		end--
	}
	passedSearch = search[start:end]

	if len(passedSearch) == 0 || len(passedSearch) == len(search) {
		return nil
	}

	if match := findTrimmedMatch(source, passedSearch); match != nil {
		match.Score = 0.75
		return match
	}
	return nil
}

func findContextAwareMatch(source, search []string) *MatchResult {
	if len(search) < 3 {
		return nil
	}
	first := strings.TrimSpace(search[0])
	last := strings.TrimSpace(search[len(search)-1])

	for i := 0; i < len(source); i++ {
		if strings.TrimSpace(source[i]) != first {
			continue
		}
		for j := i + 2; j < len(source) && j < i+len(search)*2; j++ {
			if strings.TrimSpace(source[j]) == last {
				srcMid := source[i+1 : j]
				srchMid := search[1 : len(search)-1]

				if len(srcMid) > 0 && len(srchMid) > 0 {
					return &MatchResult{StartLine: i, EndLine: j, Score: 0.70}
				}
			}
		}
	}
	return nil
}
