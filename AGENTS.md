# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-26

## Active Technologies
- Go 1.26+
- Cobra (CLI)
- Bubble Tea v2 + Bubbles v2 + Lip Gloss v2 (TUI)
- modernc.org/sqlite (profile DB)
- OpenAI-compatible provider adapter
- pure-Go HNSW (`internal/vector/hnsw`)
- golang.org/x/mod/semver (versioning)
- Profile storage: `~/.noto/profiles/<profile>/memory.db`, `~/.noto/profiles/<profile>/memory.vec`, profile metadata files
- Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for conversation/message data and input history (009-messenger-chat-ui)

## Project Structure

```text
src/
tests/
```

## Commands

`make build` - Build the project
`make test` - Run tests
`make lint` - Run linters
`make fmt` - Format code
`make vet` - Run `go vet`
`make tidy` - Run `go mod tidy`
`make clean` - Clean build artifacts
`make run` - Run the application

## Code Style

Go 1.26+: Follow standard conventions

## Recent Changes
- 009-messenger-chat-ui: Added Go 1.26+ + Cobra CLI, Bubble Tea v2, Bubbles v2 (textarea, viewport), Lip Gloss v2, modernc.org/sqlite
- 009-messenger-chat-ui: Added Go 1.26+ + Cobra CLI, Bubble Tea v2, Bubbles v2 (textarea, viewport), Lip Gloss v2, modernc.org/sqlite
- 008-note-extraction-strategy: Added Go 1.26 + Cobra, Bubble Tea v2, Bubbles v2, Lip Gloss v2, modernc.org/sqlite

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read:
<!-- SPECKIT END -->
