package sbe

import (
	"fmt"
	"os"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/fsutil"
)

// ApplyEditBlocks applies a list of edit blocks and returns total replacements.
func ApplyEditBlocks(blocks []EditBlock) (int, error) {
	total := 0
	for _, block := range blocks {
		replaced, err := applyBlock(block)
		if err != nil {
			return total, err
		}
		total += replaced
	}
	return total, nil
}

func applyBlock(block EditBlock) (int, error) {
	if strings.TrimSpace(block.FilePath) == "" {
		return 0, fmt.Errorf("file path is required")
	}
	if len(block.Search) == 0 {
		return 0, fmt.Errorf("search block is empty for %s", block.FilePath)
	}

	contentBytes, err := os.ReadFile(block.FilePath)
	if err != nil {
		return 0, fmt.Errorf("read file error: %w", err)
	}
	newline := detectNewlineForWrite(contentBytes)
	content := normalizeNewlines(string(contentBytes))
	sourceLines := strings.Split(content, "\n")

	if block.ReplaceAll {
		updated, replacements, err := replaceAllMatches(sourceLines, block.Search, block.Replace, block.FilePath)
		if err != nil {
			return 0, err
		}
		if err := writeLines(block.FilePath, updated, newline); err != nil {
			return 0, err
		}
		return replacements, nil
	}

	match := FindBestMatch(sourceLines, block.Search)
	if match == nil {
		return 0, fmt.Errorf("could not find search block in %s", block.FilePath)
	}

	updated := applyReplacement(sourceLines, match, block.Replace)
	if err := writeLines(block.FilePath, updated, newline); err != nil {
		return 0, err
	}

	return 1, nil
}

func replaceAllMatches(sourceLines, searchLines, replaceLines []string, filePath string) ([]string, int, error) {
	replacements := 0
	start := 0

	for start <= len(sourceLines) {
		match := FindBestMatch(sourceLines[start:], searchLines)
		if match == nil {
			break
		}
		match.StartLine += start
		match.EndLine += start

		sourceLines = applyReplacement(sourceLines, match, replaceLines)
		replacements++
		start = match.StartLine + len(replaceLines)
	}

	if replacements == 0 {
		return nil, 0, fmt.Errorf("could not find search block in %s", filePath)
	}

	return sourceLines, replacements, nil
}

func applyReplacement(sourceLines []string, match *MatchResult, replacement []string) []string {
	updated := append([]string{}, sourceLines[:match.StartLine]...)
	updated = append(updated, replacement...)
	if match.EndLine+1 < len(sourceLines) {
		updated = append(updated, sourceLines[match.EndLine+1:]...)
	}
	return updated
}

func detectNewlineForWrite(content []byte) string {
	if len(content) == 0 {
		return "\n"
	}
	crlf := 0
	bareLF := 0
	for i := 0; i < len(content); i++ {
		if content[i] != '\n' {
			continue
		}
		if i > 0 && content[i-1] == '\r' {
			crlf++
		} else {
			bareLF++
		}
	}
	if crlf > 0 && bareLF == 0 {
		return "\r\n"
	}
	return "\n"
}

func normalizeNewlines(value string) string {
	if value == "" {
		return value
	}
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func writeLines(filePath string, lines []string, newline string) error {
	if newline != "\r\n" {
		newline = "\n"
	}
	output := strings.Join(lines, newline)
	if err := fsutil.AtomicWriteFile(filePath, []byte(output), 0644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}
	return nil
}
