package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const maxDocumentBytes = 2 << 20

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "planreader: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("planreader", flag.ContinueOnError)
	flags.SetOutput(stderr)
	depth := flags.String("depth", "working", "narration depth: briefing, working, or full")
	audience := flags.String("audience", "software developer who may not know internal systems, identity terminology, or Compound Engineering conventions", "what the narration may assume about the listener")
	provider := flags.String("provider", "claude", "approved AI provider: claude or codex")
	claudeCommand := flags.String("claude", "claude", "path to the company-authenticated Claude Code executable")
	codexCommand := flags.String("codex", "codex", "path to the company-authenticated Codex executable")
	timeout := flags.Duration("timeout", 3*time.Minute, "maximum time to wait for the AI provider")
	noOpen := flags.Bool("no-open", false, "print the reader URL without opening a browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: planreader [options] DOCUMENT.md")
		flags.PrintDefaults()
		return errors.New("provide exactly one Markdown document")
	}

	depthPrompt, ok := map[string]string{
		"briefing": "a quick briefing of about 3 to 5 minutes",
		"working":  "a working understanding of about 10 to 15 minutes",
		"full":     "a complete spoken companion that keeps all consequential detail",
	}[*depth]
	if !ok {
		return fmt.Errorf("unknown depth %q; use briefing, working, or full", *depth)
	}
	if strings.TrimSpace(*audience) == "" {
		return errors.New("audience cannot be empty")
	}

	filePath, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		return fmt.Errorf("resolving document path: %w", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening document: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspecting document: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("document must be a regular file")
	}
	if info.Size() > maxDocumentBytes {
		return fmt.Errorf("document is %.1f MB; the current limit is 2 MB", float64(info.Size())/(1<<20))
	}
	markdown, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		return fmt.Errorf("reading document: %w", err)
	}
	if len(markdown) > maxDocumentBytes {
		return errors.New("document exceeds the current 2 MB limit")
	}
	if strings.TrimSpace(string(markdown)) == "" {
		return errors.New("document is empty")
	}

	command := *claudeCommand
	if *provider == "codex" {
		command = *codexCommand
	} else if *provider != "claude" {
		return fmt.Errorf("unknown provider %q; use claude or codex", *provider)
	}
	resolvedCommand, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("finding %s: %w; install it and sign in with the company-approved account", *provider, err)
	}

	sources := splitMarkdownSections(string(markdown))
	renderedSources, err := renderSourceSections(sources)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(stdout, "Creating a %s narration with %s…\n", *depth, *provider)
	var runner interface {
		Generate(context.Context, string, ReaderOptions) (Narration, error)
	}
	if *provider == "codex" {
		runner = CodexRunner{Command: resolvedCommand, Timeout: *timeout}
	} else {
		runner = ClaudeRunner{Command: resolvedCommand, Timeout: *timeout}
	}
	narration, err := runner.Generate(ctx, string(markdown), ReaderOptions{
		Depth:    depthPrompt,
		Audience: *audience,
		Sections: sources,
	})
	if err != nil {
		return err
	}
	if err := validateSourceMappings(narration, sources); err != nil {
		return fmt.Errorf("Claude returned an invalid source map: %w", err)
	}

	document := ReaderDocument{
		FileName:  filepath.Base(filePath),
		Narration: narration,
		Sources:   renderedSources,
	}
	url, server, err := startReaderServer(document)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(stdout, "Reader ready: %s\n", url)
	fmt.Fprintln(stdout, "Press Control-C when you are finished.")
	if !*noOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(stderr, "Could not open the browser automatically: %v\n", err)
		}
	}

	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return nil
}
