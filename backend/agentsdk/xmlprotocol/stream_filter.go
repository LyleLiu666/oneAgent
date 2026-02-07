package xmlprotocol

import (
	"strings"
	"unicode/utf8"
)

type streamFilter struct {
	pending       string
	inToolData    bool
	inThinking    bool
	thinkingClose string

	code     markdownCodeState
	codeInit bool
}

const (
	toolDataStart = "<tool_data"
	toolDataEnd   = "</tool_data>"
	thinkingStart = "<thinking"
	thinkingEnd   = "</thinking>"
	thinkStart    = "<think"
	thinkEnd      = "</think>"

	maxSuppressedTail = 64
)

func (f *streamFilter) Feed(chunk string) string {
	if chunk == "" {
		return ""
	}
	f.pending += chunk

	return f.drain(false)
}

func (f *streamFilter) Flush() string {
	return f.drain(true)
}

func (f *streamFilter) drain(final bool) string {
	if f.pending == "" {
		return ""
	}
	if !f.codeInit {
		f.code = newMarkdownCodeState()
		f.codeInit = true
	}

	var out strings.Builder

outer:
	for {
		if f.inToolData {
			endIdx := indexCaseInsensitive(f.pending, toolDataEnd)
			if endIdx == -1 {
				if final {
					f.pending = ""
					f.inToolData = false
					break
				}
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
				if final {
					f.pending = ""
					f.inThinking = false
					f.thinkingClose = ""
					break
				}
				f.pending = keepTail(f.pending, maxSuppressedTail)
				break
			}
			f.pending = f.pending[endIdx+len(f.thinkingClose):]
			f.inThinking = false
			f.thinkingClose = ""
			continue
		}

		if f.pending == "" {
			break
		}

		for i := 0; i < len(f.pending); {
			b := f.pending[i]

			if b == '`' || b == '~' {
				runLen := 1
				for i+runLen < len(f.pending) && f.pending[i+runLen] == b {
					runLen++
				}
				if !final && i+runLen == len(f.pending) {
					f.pending = f.pending[i:]
					break outer
				}

				f.code.handleDelimiterRun(b, runLen)
				out.WriteString(f.pending[i : i+runLen])
				i += runLen
				continue
			}

			if b == '<' && !f.code.inCode() {
				rem := f.pending[i:]

				// Close tags (fixed length) can be stripped without scanning for '>'.
				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, toolDataEnd); partial {
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					f.pending = rem[len(toolDataEnd):]
					continue outer
				}

				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, thinkingEnd); partial {
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					f.pending = rem[len(thinkingEnd):]
					continue outer
				}

				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, thinkEnd); partial {
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					f.pending = rem[len(thinkEnd):]
					continue outer
				}

				// Open tags require a complete start tag (up to '>') before we enter suppression mode.
				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, toolDataStart); partial {
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					startTagEndRel := strings.IndexByte(rem, '>')
					if startTagEndRel == -1 {
						if !final {
							f.pending = f.pending[i:]
							break outer
						}
					} else {
						f.pending = rem[startTagEndRel+1:]
						f.inToolData = true
						continue outer
					}
				}

				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, thinkingStart); partial {
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					startTagEndRel := strings.IndexByte(rem, '>')
					if startTagEndRel == -1 {
						if !final {
							f.pending = f.pending[i:]
							break outer
						}
					} else {
						f.pending = rem[startTagEndRel+1:]
						f.inThinking = true
						f.thinkingClose = thinkingEnd
						continue outer
					}
				}

				if full, partial := hasPrefixOrPartialCaseInsensitive(rem, thinkStart); partial {
					// Could also be a partial <thinking...> start tag.
					if !final {
						f.pending = f.pending[i:]
						break outer
					}
				} else if full {
					// Avoid treating <thinking...> as <think...>.
					if !hasPrefixCaseInsensitive(rem, thinkingStart) {
						startTagEndRel := strings.IndexByte(rem, '>')
						if startTagEndRel == -1 {
							if !final {
								f.pending = f.pending[i:]
								break outer
							}
						} else {
							f.pending = rem[startTagEndRel+1:]
							f.inThinking = true
							f.thinkingClose = thinkEnd
							continue outer
						}
					}
				}
			}

			out.WriteByte(b)
			f.code.stepByte(b)
			i++
		}

		// Fully consumed pending without entering suppression mode.
		f.pending = ""
		break
	}

	return out.String()
}

func keepTail(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	start := len(value) - max
	start = floorToRuneStart(value, start)
	return value[start:]
}

func floorToRuneStart(s string, idx int) int {
	if idx <= 0 {
		return 0
	}
	if idx >= len(s) {
		return len(s)
	}
	for idx > 0 && utf8.RuneStart(s[idx]) == false {
		idx--
	}
	return idx
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

func hasPrefixOrPartialCaseInsensitive(s, prefix string) (full bool, partial bool) {
	if s == "" {
		return false, false
	}
	if len(s) >= len(prefix) {
		if hasPrefixCaseInsensitive(s, prefix) {
			return true, false
		}
		return false, false
	}

	// s is shorter than prefix: check whether s could be a prefix fragment.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false, false
		}
	}
	return false, true
}
