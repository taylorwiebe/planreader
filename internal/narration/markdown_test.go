package narration

import (
	"strings"
	"testing"
)

func TestSplitMarkdownSections(t *testing.T) {
	input := `Opening text.

# Main plan

Summary text.

## Requirements

- First requirement
- Second requirement
`

	got := splitMarkdownSections(input)
	if len(got) != 3 {
		t.Fatalf("len(splitMarkdownSections()) = %d, want 3", len(got))
	}
	if got[0].ID != "source-0" || got[0].Heading != "Introduction" {
		t.Fatalf("first section = %#v", got[0])
	}
	if got[1].ID != "source-1" || got[1].Heading != "Main plan" {
		t.Fatalf("second section = %#v", got[1])
	}
	if got[2].ID != "source-2" || got[2].Heading != "Requirements" {
		t.Fatalf("third section = %#v", got[2])
	}
}

func TestSplitMarkdownKeepsFencedCodeHeadingText(t *testing.T) {
	input := "# Outside\n\n```markdown\n# Not a heading\n```\n"
	got := splitMarkdownSections(input)
	if len(got) != 1 {
		t.Fatalf("len(splitMarkdownSections()) = %d, want 1", len(got))
	}
}

func TestSplitMarkdownOmitsYAMLFrontMatter(t *testing.T) {
	input := "---\ntitle: Secret plan\ntype: feat\n---\n\n# Visible title\n\nBody.\n"
	got := splitMarkdownSections(input)
	if len(got) != 1 {
		t.Fatalf("len(splitMarkdownSections()) = %d, want 1", len(got))
	}
	if strings.Contains(got[0].Markdown, "title: Secret plan") {
		t.Fatalf("section contains frontmatter: %q", got[0].Markdown)
	}
}
