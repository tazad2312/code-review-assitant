package review

import (
	"fmt"
	"strings"
)

const approximateCharactersPerToken = 4

// ChunkDiff keeps file and hunk boundaries intact where possible. Very large
// hunks are split on lines and retain the file/hunk headers as context.
func ChunkDiff(diff string, tokenBudget int) ([]string, error) {
	if tokenBudget < 1 {
		return nil, fmt.Errorf("token budget must be positive")
	}
	maxCharacters := tokenBudget * approximateCharactersPerToken
	files := splitDiffFiles(diff)
	chunks := make([]string, 0, len(files))
	for _, file := range files {
		chunks = append(chunks, splitFile(file, maxCharacters)...)
	}
	return chunks, nil
}

func splitDiffFiles(diff string) []string {
	var files []string
	var current strings.Builder
	for _, line := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") && current.Len() > 0 {
			files = append(files, current.String())
			current.Reset()
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		files = append(files, current.String())
	}
	return files
}

func splitFile(file string, maxCharacters int) []string {
	lines := strings.SplitAfter(file, "\n")
	headerEnd := 0
	for headerEnd < len(lines) && !strings.HasPrefix(lines[headerEnd], "@@") {
		headerEnd++
	}
	header := strings.Join(lines[:headerEnd], "")
	if headerEnd == len(lines) {
		return splitLines("", lines, maxCharacters)
	}

	var chunks []string
	var current strings.Builder
	current.WriteString(header)
	for i := headerEnd; i < len(lines); {
		if strings.HasPrefix(lines[i], "@@") && current.Len() > len(header) && current.Len()+len(lines[i]) > maxCharacters {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(header)
		}
		if current.Len()+len(lines[i]) > maxCharacters && current.Len() > len(header) {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(header)
		}
		if current.Len()+len(lines[i]) > maxCharacters && current.Len() == len(header) {
			prefix := header
			if strings.HasPrefix(lines[i], "@@") {
				prefix += lines[i]
				i++
			}
			pieces := splitLines(prefix, lines[i:], maxCharacters)
			chunks = append(chunks, pieces...)
			return chunks
		}
		current.WriteString(lines[i])
		i++
	}
	if current.Len() > len(header) || len(chunks) == 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func splitLines(prefix string, lines []string, maxCharacters int) []string {
	var chunks []string
	var current strings.Builder
	current.WriteString(prefix)
	for _, line := range lines {
		if current.Len()+len(line) > maxCharacters && current.Len() > len(prefix) {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(prefix)
		}
		current.WriteString(line)
	}
	if current.Len() > len(prefix) || len(chunks) == 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}
