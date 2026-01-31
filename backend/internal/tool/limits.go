package tool

import "unicode/utf8"

const (
	maxEditOpsPerCall        = 10
	maxEditSnippetRunes      = 3000
	maxEditTotalRunesPerCall = 12000
	// write_file is frequently used for generating HTML/markdown artifacts; keep this large enough
	// to avoid excessive append loops under XML tool protocol, but still bounded for safety.
	maxWriteFileRunesPerCall   = 200000
	maxWriteFilePathRunesLimit = 512
)

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
}

func truncateToRunes(value string, maxRunes int) (string, bool, int) {
	original := utf8.RuneCountInString(value)
	if maxRunes <= 0 {
		return "", original > 0, original
	}
	if original <= maxRunes {
		return value, false, original
	}

	n := 0
	end := 0
	for i := range value {
		if n == maxRunes {
			end = i
			break
		}
		n++
	}
	if end == 0 {
		end = len(value)
	}
	return value[:end], true, original
}
