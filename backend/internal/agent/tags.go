package agent

import (
	"html"
	"strings"
)

// ExtractLatestTagBlock returns the last occurrence of a `<tag>...</tag>` block (best-effort).
//
// It is intentionally tolerant:
// - If the closing tag is missing, it returns from the last `<tag` to end-of-text.
// - It is case-insensitive for tag matching.
func ExtractLatestTagBlock(text string, tag string) (string, bool) {
	text = strings.TrimSpace(text)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if text == "" || tag == "" {
		return "", false
	}

	lower := strings.ToLower(text)
	closeTag := "</" + tag + ">"

	end := strings.LastIndex(lower, closeTag)
	if end == -1 {
		start := strings.LastIndex(lower, "<"+tag)
		if start == -1 {
			return "", false
		}
		return strings.TrimSpace(text[start:]), true
	}
	end = end + len(closeTag)

	start := strings.LastIndex(lower[:end], "<"+tag)
	if start == -1 {
		return "", false
	}
	return strings.TrimSpace(text[start:end]), true
}

// ExtractTagValue extracts the inner text of `<tag>...</tag>` from a block (best-effort).
//
// If the closing tag is missing, it will stop at the next `<stopTag` if stopTags is provided,
// otherwise it reads to the end of block.
//
// It also tolerates CDATA and HTML entity escaping by default.
func ExtractTagValue(block string, tag string, stopTags []string) (string, bool) {
	block = strings.TrimSpace(block)
	tag = strings.ToLower(strings.TrimSpace(tag))
	if block == "" || tag == "" {
		return "", false
	}

	lower := strings.ToLower(block)

	start := strings.Index(lower, "<"+tag)
	if start == -1 {
		return "", false
	}
	startTagEndRel := strings.Index(block[start:], ">")
	if startTagEndRel == -1 {
		return "", false
	}
	startTagEnd := start + startTagEndRel + 1

	endTag := "</" + tag + ">"
	endRel := strings.Index(lower[startTagEnd:], endTag)
	end := -1
	if endRel != -1 {
		end = startTagEnd + endRel
	} else if len(stopTags) > 0 {
		next := len(block)
		for _, stop := range stopTags {
			stop = strings.ToLower(strings.TrimSpace(stop))
			if stop == "" || stop == tag {
				continue
			}
			idx := strings.Index(lower[startTagEnd:], "<"+stop)
			if idx == -1 {
				continue
			}
			abs := startTagEnd + idx
			if abs < next {
				next = abs
			}
		}
		if next != len(block) {
			end = next
		}
	}

	if end == -1 {
		end = len(block)
	}
	if end < startTagEnd {
		return "", false
	}

	value := strings.TrimSpace(block[startTagEnd:end])
	if value == "" {
		return "", true
	}

	// Default-CDATA behavior: accept raw text, but also tolerate explicit/malformed CDATA.
	if strings.HasPrefix(value, "<![CDATA[") {
		if end := strings.Index(value, "]]>"); end != -1 {
			value = value[len("<![CDATA["):end]
		} else {
			value = value[len("<![CDATA["):]
		}
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(value), true
}

