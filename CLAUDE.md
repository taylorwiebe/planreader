# Planreader project guidance

Planreader is a local Go application that turns Markdown into a private spoken companion. Build and test it with the vendored dependency set:

```sh
go test -mod=vendor ./...
go vet -mod=vendor ./...
go build -mod=vendor ./...
```

Use the `read-with-planreader` skill whenever a user asks to open, simplify, narrate, listen to, or make a spoken briefing from a Markdown document. Keep the narration prompt and quality rules in `internal/narration`; do not duplicate them in agent instructions.
