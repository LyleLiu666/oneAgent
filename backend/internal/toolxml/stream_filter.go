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
			lower := strings.ToLower(f.pending)
			endIdx := strings.Index(lower, toolDataEnd)
			if endIdx == -1 {
				f.pending = keepTail(f.pending, maxSuppressedTail)
				break
			}
			f.pending = f.pending[endIdx+len(toolDataEnd):]
			f.inToolData = false
			continue
		}

		if f.inThinking {
			lower := strings.ToLower(f.pending)
			endIdx := strings.Index(lower, f.thinkingClose)
			if endIdx == -1 {
				f.pending = keepTail(f.pending, maxSuppressedTail)
				break
			}
			f.pending = f.pending[endIdx+len(f.thinkingClose):]
			f.inThinking = false
			f.thinkingClose = ""
			continue
		}

		lower := strings.ToLower(f.pending)

		idxTool := strings.Index(lower, toolDataStart)
		idxThinking := strings.Index(lower, thinkingStart)
		idxThink := strings.Index(lower, thinkStart)

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

		startTagEndRel := strings.Index(f.pending[idx:], ">")
		if startTagEndRel == -1 {
			// Wait for the rest of the tag.
			f.pending = f.pending[idx:]
			break
		}
		startTagEnd := idx + startTagEndRel + 1
		startLower := strings.ToLower(f.pending[idx:startTagEnd])

		f.pending = f.pending[startTagEnd:]
		switch {
		case strings.HasPrefix(startLower, toolDataStart):
			f.inToolData = true
		case strings.HasPrefix(startLower, thinkingStart):
			f.inThinking = true
			f.thinkingClose = thinkingEnd
		case strings.HasPrefix(startLower, thinkStart):
			// Avoid treating <thinking...> as <think...>.
			if strings.HasPrefix(startLower, thinkingStart) {
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
