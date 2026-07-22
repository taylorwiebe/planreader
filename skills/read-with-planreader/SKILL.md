---
name: read-with-planreader
description: Turn a local Markdown document into Planreader's clear, private spoken companion and open the reader. Use when the user asks to open, read, listen to, simplify, narrate, brief, or make an audiobook from a Markdown file, plan, specification, or technical document with Planreader.
---

# Read with Planreader

Use Planreader as the single source of truth for preparing the human-readable narration. Do not summarize the document separately or reproduce Planreader's narration prompt in this skill.

## Run the document

1. Locate the Planreader repository with `git rev-parse --show-toplevel` from the user's current worktree. Confirm it is Planreader by checking for `go.mod` and `cmd/root.go`.
2. Resolve the requested Markdown path. Treat relative paths as relative to the user's current working directory.
3. Confirm that the source is a readable regular `.md` file. Never modify it.
4. Select the provider that matches the current agent:
   - In Claude Code, pass `--provider claude`.
   - In Codex, pass `--provider codex`.
   - Never switch providers to bypass authentication, quota, or policy failures unless the user explicitly asks.
5. From the Planreader repository, run:

   ```sh
   go run -mod=vendor . --provider PROVIDER --depth working DOCUMENT.md
   ```

   Use `briefing` for a quick overview, `working` by default, and `full` only when the user requests comprehensive detail. Add `--audience` when the user states what they know or which jargon needs explanation.
6. Keep the process running. Wait for `Reader ready:` and open or navigate to that URL when the environment permits it. If Planreader already opened the browser, do not open a duplicate tab.
7. Tell the user that the reader is ready and which provider was used. Do not dump generation logs or narration into chat.

## Preserve narration quality

Let Planreader create the actual document the user will hear. Its application-owned prompt resolves references, explains jargon, preserves consequential facts, identifies assumptions and uncertainty, retains valuable diagrams, adds recall pauses, and separates decisions, actions, and verification items. If these requirements need to change, update and test the narration code rather than overriding it here.

## Reuse prepared narration

Use `--prepared DATA.json` only when the user asks to avoid regeneration and the prepared data is known to belong to the same source document. Never substitute unrelated or stale prepared narration.

## Handle failures

- If the selected provider is missing, signed out, rate-limited, or out of quota, report that exact condition and leave the provider unchanged.
- If browser opening fails but `Reader ready:` was printed, give the user the local URL.
- If narration generation fails, do not fall back to a chat summary as though the reader succeeded.
- Treat the browser's computer-voice fallback as a speech-setting issue, not a narration-generation failure.
