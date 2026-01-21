package sbe

import (
	"fmt"
	"os"
	"strings"
)

// ApplyEdits parses command and applies changes to files
func ApplyEdits(rawCommand string) error {
	blocks, err := ParseSmartEditCommand(rawCommand)
	if err != nil {
		return err
	}

	for _, block := range blocks {
		// Read file
		contentBytes, err := os.ReadFile(block.FilePath)
		if err != nil {
			return fmt.Errorf("read file error: %w", err)
		}
		sourceLines := strings.Split(string(contentBytes), "\n")

		// Fuzzy Find
		match := FindBestMatch(sourceLines, block.Search)
		if match == nil {
			return fmt.Errorf("could not find search block in %s", block.FilePath)
		}

		// Apply Replacement
		// Construct new lines: [Pre-match] + [Replacement] + [Post-match]
		newLines := append([]string{}, sourceLines[:match.StartLine]...)
		newLines = append(newLines, block.Replace...)
		if match.EndLine+1 < len(sourceLines) {
			newLines = append(newLines, sourceLines[match.EndLine+1:]...)
		}

		// Write back
		output := strings.Join(newLines, "\n")
		err = os.WriteFile(block.FilePath, []byte(output), 0644)
		if err != nil {
			return fmt.Errorf("write file error: %w", err)
		}
	}
	return nil
}
