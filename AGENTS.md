# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-13

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

# Add commands for Go 1.26+

## Code Style

Go 1.26+: Follow standard conventions

## Recent Changes

- 007-release-pipelines: Added Go 1.26 + Cobra, Bubble Tea/Bubbles, Lip Gloss, modernc.org/sqlite, golang.org/x/mod/semver (new)
- 005-memory-context: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]
- 004-bubbletea-tui: Added Go 1.26+ + Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (CLI)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
