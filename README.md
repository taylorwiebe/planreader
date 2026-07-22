# Planreader

Planreader turns a local Markdown document into a simpler spoken companion.
It uses the Claude Code or Codex installation and authentication already configured on your computer, then serves a private local reader with sentence highlighting and browser speech.

## Requirements

- Go 1.25 or newer to build from source
- Claude Code or Codex installed and signed in through your company-approved account
- A browser with the Web Speech API, such as Safari or Chrome

## Build

```bash
go build -o planreader .
```

## Use

```bash
./planreader path/to/plan.md
```

Choose the narration depth:

```bash
./planreader --depth briefing path/to/plan.md
./planreader --depth working path/to/plan.md
./planreader --depth full path/to/plan.md
```

Use your signed-in Codex account instead of Claude Code:

```bash
./planreader --provider codex --depth briefing path/to/plan.md
```

Tell the narrator what it may assume:

```bash
./planreader --audience "I know Go and Rails. Explain identity systems and internal Compound Engineering terminology." path/to/plan.md
```

Planreader prints the private local URL and normally opens it in your default browser.
Press Control-C in the terminal when you are finished.

## Data boundaries

- The original file is read in place and is never modified.
- Document text is passed to the selected AI command through standard input, not command arguments.
- Claude Code runs without tools or session persistence. Codex runs ephemerally in a temporary read-only workspace with structured output.
- The reader server binds only to `127.0.0.1` and uses a random URL token.
- The browser performs speech synthesis; Planreader does not send narration to a separate speech provider.
- Planreader does not persist the document, narration, audio, or reading progress.

The exact model route and retention guarantees still depend on your organization's Claude Code or Codex configuration and policies.

## Development

```bash
go test ./...
go vet ./...
```
