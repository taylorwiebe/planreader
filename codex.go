package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CodexRunner struct {
	Command string
	Timeout time.Duration
	Env     []string
}

func (r CodexRunner) Generate(ctx context.Context, markdown string, options ReaderOptions) (Narration, error) {
	command := r.Command
	if command == "" {
		command = "codex"
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "planreader-codex-")
	if err != nil {
		return Narration{}, fmt.Errorf("creating temporary Codex workspace: %w", err)
	}
	defer os.RemoveAll(tempDir)

	schemaPath := filepath.Join(tempDir, "narration-schema.json")
	resultPath := filepath.Join(tempDir, "narration.json")
	if err := os.WriteFile(schemaPath, []byte(narrationSchema), 0o600); err != nil {
		return Narration{}, fmt.Errorf("writing Codex output schema: %w", err)
	}

	cmd := exec.CommandContext(ctx, command, "exec",
		"--sandbox", "read-only",
		"--ephemeral",
		"--skip-git-repo-check",
		"--color", "never",
		"--output-schema", schemaPath,
		"--output-last-message", resultPath,
		"-",
	)
	cmd.Dir = tempDir
	cmd.Stdin = strings.NewReader(createNarrationPrompt(markdown, options))
	if r.Env != nil {
		cmd.Env = r.Env
	}
	var stderr bytes.Buffer
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Narration{}, fmt.Errorf("Codex did not finish within %s", timeout)
		}
		message := strings.TrimSpace(stderr.String())
		if len(message) > 1_000 {
			message = message[len(message)-1_000:]
		}
		if message == "" {
			message = err.Error()
		}
		return Narration{}, fmt.Errorf("Codex failed: %s", message)
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return Narration{}, fmt.Errorf("reading Codex output: %w", err)
	}
	var narration Narration
	if err := json.Unmarshal(data, &narration); err != nil {
		return Narration{}, fmt.Errorf("decoding Codex narration: %w", err)
	}
	if err := validateNarration(narration); err != nil {
		return Narration{}, err
	}
	return narration, nil
}
