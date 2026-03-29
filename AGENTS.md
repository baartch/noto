# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-28

## Active Technologies
- Go 1.23+ + Cobra (CLI command surface), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)
- Local SQLite per profile (`~/.noto/profiles/<profile>/memory.db`) + profile-local (001-build-profile-memory-cli)
- Go 1.23+ + Cobra (command definitions), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)
- Go 1.23+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation (to be added under `internal/vector/hnsw` to avoid cgo) (001-build-profile-memory-cli)
- SQLite per profile + single-file vector index `~/.noto/profiles/<profile>/memory.vec` (001-build-profile-memory-cli)
- Go 1.23+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation under `internal/vector/hnsw` (no cgo) (001-build-profile-memory-cli)

- Go 1.23+ + Cobra (CLI commands), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.23+

## Code Style

Go 1.23+: Follow standard conventions

## Recent Changes
- 001-build-profile-memory-cli: Added Go 1.23+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation under `internal/vector/hnsw` (no cgo)
- 001-build-profile-memory-cli: Added Go 1.23+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation (to be added under `internal/vector/hnsw` to avoid cgo)
- 001-build-profile-memory-cli: Added Go 1.23+ + Cobra (command definitions), Bubble Tea + Bubbles + Lip Gloss (TUI),


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
