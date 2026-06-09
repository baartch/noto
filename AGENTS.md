# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-27

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
- Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for notes/cache + profile-local files (`~/.noto/profiles/<profile>/prompts/*.md`, `memory.vec`) (005-memory-context)
- Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) + profile-local files (`memory.vec`, prompt files) (004-bubbletea-tui)

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
- 004-bubbletea-tui: Added Go 1.26+ + Cobra CLI, Bubble Tea v2, Bubbles v2, Lip Gloss v2, OpenAI-compatible provider adapter
- 005-memory-context: Added Go 1.26+ + Cobra CLI, Bubble Tea v2, Bubbles v2, Lip Gloss v2, modernc.org/sqlite, OpenAI-compatible provider adapter, internal pure-Go HNSW index
- 009-messenger-chat-ui: Added Go 1.26+ + Cobra CLI, Bubble Tea v2, Bubbles v2 (textarea, viewport), Lip Gloss v2, modernc.org/sqlite

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->
