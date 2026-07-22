package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexRunnerGenerate(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.txt")
	argsPath := filepath.Join(tempDir, "args.txt")
	commandPath := filepath.Join(tempDir, "fake-codex")
	response := `{"title":"Codex plan","sections":[{"id":"overview","heading":"Overview","source_section_ids":["source-0"],"sentences":["The plan has one main idea."]}]}`
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$ARGS_PATH\"\ncat > \"$INPUT_PATH\"\nwhile [ \"$#\" -gt 0 ]; do\n  if [ \"$1\" = \"--output-last-message\" ]; then\n    shift\n    printf '%s' '" + response + "' > \"$1\"\n  fi\n  shift\ndone\n"
	if err := os.WriteFile(commandPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	runner := CodexRunner{
		Command: commandPath,
		Timeout: 5 * time.Second,
		Env: append(os.Environ(),
			"INPUT_PATH="+inputPath,
			"ARGS_PATH="+argsPath,
		),
	}
	narration, err := runner.Generate(context.Background(), "# Plan\nCompany content", ReaderOptions{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if narration.Title != "Codex plan" {
		t.Fatalf("title = %q, want Codex plan", narration.Title)
	}

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(input), "Company content") {
		t.Fatal("Codex stdin did not include the document")
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"exec", "--sandbox read-only", "--ephemeral", "--output-schema", "--output-last-message"} {
		if !strings.Contains(string(args), flag) {
			t.Fatalf("Codex arguments %q do not contain %s", args, flag)
		}
	}
}

func TestCodexRunnerReportsCommandFailureWithoutDocument(t *testing.T) {
	commandPath := filepath.Join(t.TempDir(), "fake-codex")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\necho 'authentication required' >&2\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	runner := CodexRunner{Command: commandPath, Timeout: 5 * time.Second}
	_, err := runner.Generate(context.Background(), "company secret", ReaderOptions{})
	if err == nil {
		t.Fatal("Generate() error = nil, want an error")
	}
	if strings.Contains(err.Error(), "company secret") {
		t.Fatalf("Generate() error leaked document content: %v", err)
	}
}
