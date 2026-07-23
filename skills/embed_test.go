package skills

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedSkillIsRepositoryIndependent(t *testing.T) {
	for _, name := range []string{"read-with-planreader/SKILL.md", "read-with-planreader/agents/openai.yaml"} {
		if _, err := fs.ReadFile(Files, name); err != nil {
			t.Fatalf("embedded %s: %v", name, err)
		}
	}
	data, err := fs.ReadFile(Files, "read-with-planreader/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "planreader ") || !strings.Contains(text, "--provider PROVIDER") || strings.Contains(text, "go run") || strings.Contains(text, "git rev-parse") {
		t.Fatalf("skill is not installed-command based:\n%s", text)
	}
}
