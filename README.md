# Planreader

Planreader turns a local Markdown document into a simpler spoken companion.
It uses the Claude Code or Codex installation and authentication already configured on your computer, then serves a private local reader with sentence highlighting. Computer speech works immediately; Apple silicon Mac users can optionally install a more natural local voice pack.

## Requirements

- Go 1.25 or newer to build from source
- Claude Code or Codex installed and signed in through your company-approved account
- A browser with the Web Speech API, such as Safari or Chrome
- Apple silicon is currently required only for optional neural voice packs

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

## Speech choices

Open **Speech settings** in the reader to choose:

- **Computer voices:** no download and the fallback on every computer.
- **Kokoro 82M:** about 190 MB; a compact local model with 28 expressive American and British English voices.

Voice packs are pinned, integrity-checked downloads from Hugging Face and use the Apache 2.0 license. They are stored in your Planreader application settings directory and can be removed from the same settings panel. Your selected speech source, voice, and speed are remembered for future documents. If an installed model is moved, deleted, corrupt, or incompatible, Planreader switches back to a computer voice and keeps reading available.

## Data boundaries

- The original file is read in place and is never modified.
- Document text is passed to the selected AI command through standard input, not command arguments.
- Claude Code runs without tools or session persistence. Codex runs ephemerally in a temporary read-only workspace with structured output.
- The reader server binds only to `127.0.0.1` and uses a random URL token.
- Computer voices use the browser's speech support. Optional neural voices synthesize locally through sherpa-onnx; narration text and generated audio are not sent to Hugging Face or another speech provider.
- Model installation contacts Hugging Face but sends no document content.
- Planreader remembers speech preferences and installed model files. It does not persist the document, narration, generated audio, or reading progress. Temporary audio is deleted when the reader closes.

The exact model route and retention guarantees still depend on your organization's Claude Code or Codex configuration and policies.

## Development

```bash
go test ./...
go vet ./...
```
