package toolxml

import "strings"

type streamFilter struct {
	pending       string
	inToolData    bool
	inThinking    bool
	thinkingClose string
}

const (
	toolDataStart = "<tool_data"
	toolDataEnd   = "</tool_data>"
	thinkingStart = "<thinking"
	thinkingEnd   = "</thinking>"
	thinkStart    = "<think"
	thinkEnd      = "</think>"

	maxSuppressedTail = 64
	maxSearchTail     = 16
)

func (f *streamFilter) Feed(chunk string) string {
	if chunk == "" {
		return ""
	}
	f.pending += chunk

	var out strings.Builder

	for {
		if f.inToolData {
			endIdx := indexCaseInsensitive(f.pending, toolDataEnd)
			if endIdx == -1 {
				f.pending = keepTail(f.pending, maxSuppressedTail)
				break
			}
			f.pending = f.pending[endIdx+len(toolDataEnd):]
			f.inToolData = false
			continue
		}

		if f.inThinking {
			endIdx := indexCaseInsensitive(f.pending, f.thinkingClose)
			if endIdx == -1 {
				f.pending = keepTail(f.pending, maxSuppressedTail)
				break
			}
			f.pending = f.pending[endIdx+len(f.thinkingClose):]
			f.inThinking = false
			f.thinkingClose = ""
			continue
		}

		idxTool := indexCaseInsensitive(f.pending, toolDataStart)
		idxThinking := indexCaseInsensitive(f.pending, thinkingStart)
		idxThink := indexCaseInsensitive(f.pending, thinkStart)

		idx := minNonNegative(idxTool, idxThinking, idxThink)
		if idx == -1 {
			if len(f.pending) <= maxSearchTail {
				break
			}
			out.WriteString(f.pending[:len(f.pending)-maxSearchTail])
			f.pending = f.pending[len(f.pending)-maxSearchTail:]
			break
		}

		out.WriteString(f.pending[:idx])

		// We found a start tag at idx.
		// Now we need to see if we have the full tag (up to '>') to verify it's not a partial match or similar?
		// Actually, the original code looked for '>' to decide if the tag is complete.
		// "startTagEndRel := strings.Index(f.pending[idx:], ">")"
		// This logic is preserved but we just work on f.pending.

		startTagEndRel := strings.Index(f.pending[idx:], ">")
		if startTagEndRel == -1 {
			// Wait for the rest of the tag.
			f.pending = f.pending[idx:]
			break
		}
		startTagEnd := idx + startTagEndRel + 1

		// Check which tag it really is.
		snippet := f.pending[idx:startTagEnd]

		f.pending = f.pending[startTagEnd:]
		switch {
		case hasPrefixCaseInsensitive(snippet, toolDataStart):
			f.inToolData = true
		case hasPrefixCaseInsensitive(snippet, thinkingStart):
			f.inThinking = true
			f.thinkingClose = thinkingEnd
		case hasPrefixCaseInsensitive(snippet, thinkStart):
			// Avoid treating <thinking...> as <think...>.
			if hasPrefixCaseInsensitive(snippet, thinkingStart) {
				f.inThinking = true
				f.thinkingClose = thinkingEnd
			} else {
				f.inThinking = true
				f.thinkingClose = thinkEnd
			}
		}
	}

	return out.String()
}

func (f *streamFilter) Flush() string {
	if f.inToolData || f.inThinking {
		f.pending = ""
		return ""
	}
	out := f.pending
	f.pending = ""
	return out
}

func minNonNegative(values ...int) int {
	min := -1
	for _, v := range values {
		if v < 0 {
			continue
		}
		if min == -1 || v < min {
			min = v
		}
	}
	return min
}

func keepTail(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}

// indexCaseInsensitive finds the index of sub in s, ignoring case.
// sub is assumed to be lower-case.
func indexCaseInsensitive(s, sub string) int {
	if sub == "" {
		return 0
	}
	if len(s) < len(sub) {
		return -1
	}

	// Brute force search is fine for short strings and specific tags we are looking for.
	// Optimizations like Boyer-Moore could be applied but likely overkill here given sub is constants.
	for i := 0; i <= len(s)-len(sub); i++ {
		if hasPrefixCaseInsensitive(s[i:], sub) {
			return i
		}
	}
	return -1
}

// hasPrefixCaseInsensitive checks if s starts with prefix, ignoring case.
// prefix is assumed to be lower-case.
func hasPrefixCaseInsensitive(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}
