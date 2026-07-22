package narration

import (
	"fmt"
	"regexp"
	"strings"
)

var markdownHeading = regexp.MustCompile(`^(#{1,6})[\t ]+(.+?)[\t ]*#*[\t ]*$`)

type SourceSection struct {
	ID       string `json:"id"`
	Heading  string `json:"heading"`
	Level    int    `json:"level"`
	Markdown string `json:"markdown"`
}

func splitMarkdownSections(markdown string) []SourceSection {
	markdown = stripYAMLFrontMatter(markdown)
	lines := strings.Split(markdown, "\n")
	sections := make([]SourceSection, 0)
	var current *SourceSection
	inFence := false

	flush := func() {
		if current == nil || strings.TrimSpace(current.Markdown) == "" {
			return
		}
		current.Markdown = strings.TrimSpace(current.Markdown)
		current.ID = fmt.Sprintf("source-%d", len(sections))
		sections = append(sections, *current)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}

		match := markdownHeading.FindStringSubmatch(line)
		if !inFence && len(match) == 3 {
			flush()
			current = &SourceSection{
				Heading: strings.TrimSpace(match[2]),
				Level:   len(match[1]),
			}
		}
		if current == nil {
			current = &SourceSection{Heading: "Introduction", Level: 1}
		}
		current.Markdown += line + "\n"
	}
	flush()

	if len(sections) == 0 && strings.TrimSpace(markdown) != "" {
		sections = append(sections, SourceSection{
			ID:       "source-0",
			Heading:  "Introduction",
			Level:    1,
			Markdown: strings.TrimSpace(markdown),
		})
	}
	return sections
}

func SplitMarkdownSections(markdown string) []SourceSection {
	return splitMarkdownSections(markdown)
}

func stripYAMLFrontMatter(markdown string) string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return normalized
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return normalized
}
