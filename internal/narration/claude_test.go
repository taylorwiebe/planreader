package narration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeRunnerGenerate(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.txt")
	argsPath := filepath.Join(tempDir, "args.txt")
	commandPath := filepath.Join(tempDir, "fake-claude")
	response := `{"structured_output":{"title":"Short plan","sections":[{"id":"overview","heading":"Overview","source_section_ids":["source-0"],"sentences":["The plan has one main idea."]}]}}`
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$ARGS_PATH\"\ncat > \"$INPUT_PATH\"\nprintf '%s' '" + response + "'\n"
	if err := os.WriteFile(commandPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	runner := ClaudeRunner{
		Command: commandPath,
		Timeout: 5 * time.Second,
		Env: append(os.Environ(),
			"INPUT_PATH="+inputPath,
			"ARGS_PATH="+argsPath,
		),
	}
	narration, err := runner.Generate(context.Background(), "# Plan\nSecret content", ReaderOptions{
		Depth:    "briefing",
		Audience: "software developer",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if narration.Title != "Short plan" {
		t.Fatalf("title = %q, want Short plan", narration.Title)
	}

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(input), "Secret content") {
		t.Fatal("Claude stdin did not include the document")
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--no-session-persistence", "--json-schema", "--tools"} {
		if !strings.Contains(string(args), flag) {
			t.Fatalf("Claude arguments %q do not contain %s", args, flag)
		}
	}
}

func TestClaudeRunnerReportsCommandFailureWithoutDocument(t *testing.T) {
	tempDir := t.TempDir()
	commandPath := filepath.Join(tempDir, "fake-claude")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\necho 'authentication required' >&2\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	runner := ClaudeRunner{Command: commandPath, Timeout: 5 * time.Second}
	_, err := runner.Generate(context.Background(), "company secret", ReaderOptions{})
	if err == nil {
		t.Fatal("Generate() error = nil, want an error")
	}
	if strings.Contains(err.Error(), "company secret") {
		t.Fatalf("Generate() error leaked document content: %v", err)
	}
}

func TestNarrationPromptProducesASelfContainedHumanBriefing(t *testing.T) {
	prompt := createNarrationPrompt("# Plan\nSee `auth.go` and requirement R-12.", ReaderOptions{})

	for _, instruction := range []string{
		"What is being proposed",
		"Why it matters",
		"Risks and unresolved questions",
		"Resolve references inline",
		"complete sentences",
		"Details worth knowing",
		"valuable technical diagram",
		"walk through its meaning",
		"Do not silently turn assumptions",
		"label it as an assumption",
	} {
		if !strings.Contains(prompt, instruction) {
			t.Fatalf("narration prompt is missing human-reader instruction %q", instruction)
		}
	}
}
