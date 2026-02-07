package xmlprotocol

// markdownCodeState tracks whether the current cursor is inside Markdown inline
// code spans (backticks) or fenced code blocks (``` / ~~~, allowing up to 3
// leading spaces).
//
// This is intentionally minimal and ASCII-focused: it only looks at `, ~, space,
// tab, and newline.
type markdownCodeState struct {
	atLineStart bool
	indent      int

	inFence     bool
	fenceMarker byte
	fenceLen    int

	inlineTicks int
}

func newMarkdownCodeState() markdownCodeState {
	return markdownCodeState{atLineStart: true}
}

func (s *markdownCodeState) inCode() bool {
	return s.inFence || s.inlineTicks > 0
}

func (s *markdownCodeState) stepByte(b byte) {
	if b == '\n' {
		s.atLineStart = true
		s.indent = 0
		return
	}

	if !s.atLineStart {
		return
	}

	switch b {
	case ' ':
		s.indent++
		if s.indent > 3 {
			s.atLineStart = false
		}
	case '\t':
		s.indent += 4
		s.atLineStart = false
	default:
		s.atLineStart = false
	}
}

func (s *markdownCodeState) handleDelimiterRun(marker byte, runLen int) {
	if s.inFence {
		if s.atLineStart && s.indent <= 3 && marker == s.fenceMarker && runLen >= s.fenceLen {
			s.inFence = false
			s.fenceMarker = 0
			s.fenceLen = 0
		}
	} else {
		if s.inlineTicks == 0 && s.atLineStart && s.indent <= 3 && runLen >= 3 {
			s.inFence = true
			s.fenceMarker = marker
			s.fenceLen = runLen
		} else if marker == '`' {
			if s.inlineTicks == 0 {
				s.inlineTicks = runLen
			} else if runLen == s.inlineTicks {
				s.inlineTicks = 0
			}
		}
	}

	// After any non-whitespace content on the line, we're no longer at the
	// "line start" prefix where fences can be started/ended.
	if s.atLineStart {
		s.atLineStart = false
	}
}

