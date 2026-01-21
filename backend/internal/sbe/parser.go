package sbe

import (
	"bufio"
	"fmt"
	"strings"
)

// ParseSmartEditCommand parses the raw shell command input
// Input format:
// apply_smart_edit <<'EOF'
// file: /path/to/file
// <<<< SEARCH
// ...
// ==== REPLACE
// ...
// >>>>
// EOF
func ParseSmartEditCommand(rawInput string) ([]EditBlock, error) {
	// 1. Heredoc stripping (simplified)
	// Remove "apply_smart_edit <<'EOF'" header and "EOF" footer
	content := rawInput
	if strings.Contains(content, "<<'EOF'") {
		parts := strings.SplitN(content, "<<'EOF'", 2)
		if len(parts) > 1 {
			content = parts[1]
		}
	}
	content = strings.TrimSuffix(strings.TrimSpace(content), "EOF")

	var blocks []EditBlock
	var currentBlock *EditBlock
	var currentSection string // "SEARCH" or "REPLACE"
	var currentFile string

	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// 2. State Machine
		if strings.HasPrefix(line, "file: ") {
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, "file: "))
			continue
		}

		if line == "<<<< SEARCH" {
			// Start new block
			currentBlock = &EditBlock{FilePath: currentFile}
			currentSection = "SEARCH"
			continue
		}

		if line == "==== REPLACE" {
			currentSection = "REPLACE"
			continue
		}

		if line == ">>>>" {
			// End block
			if currentBlock != nil {
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
			}
			currentSection = ""
			continue
		}

		// Content accumulation
		if currentBlock != nil {
			if currentSection == "SEARCH" {
				currentBlock.Search = append(currentBlock.Search, line)
			} else if currentSection == "REPLACE" {
				currentBlock.Replace = append(currentBlock.Replace, line)
			}
		}
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no valid edit blocks found")
	}

	return blocks, nil
}
