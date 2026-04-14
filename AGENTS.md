# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-14

## Active Technologies
- Go 1.26+
- Cobra (CLI)
- Bubble Tea + Bubbles + Lip Gloss (TUI)
- modernc.org/sqlite (profile DB)
- OpenAI-compatible provider adapter
- pure-Go HNSW (`internal/vector/hnsw`)
- golang.org/x/mod/semver (versioning)
- Profile storage: `~/.noto/profiles/<profile>/memory.db`, `~/.noto/profiles/<profile>/memory.vec`, profile metadata files

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
- 007-release-pipelines: Added Go 1.26 + Cobra, Bubble Tea/Bubbles, Lip Gloss, modernc.org/sqlite, golang.org/x/mod/semver (new)
- 006-settings-dialog: Added Go 1.26+ + Bubble Tea v2, Bubbles v2, Lip Gloss v2, Cobra

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
