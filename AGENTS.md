# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-29

## Active Technologies

- Go 1.26+ + Cobra (CLI command surface), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)
- Local SQLite per profile (`~/.noto/profiles/<profile>/memory.db`) + profile-local (001-build-profile-memory-cli)
- Go 1.26+ + Cobra (command definitions), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)
- Go 1.26+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation (to be added under `internal/vector/hnsw` to avoid cgo) (001-build-profile-memory-cli)
- SQLite per profile + single-file vector index `~/.noto/profiles/<profile>/memory.vec` (001-build-profile-memory-cli)
- Go 1.26+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation under `internal/vector/hnsw` (no cgo) (001-build-profile-memory-cli)
- Go 1.26+ + Cobra (CLI), Bubble Tea/Bubbles/Lip Gloss (TUI) (002-portable-profiles)
- SQLite (per-profile DB), profile directory metadata files (002-portable-profiles)

- Go 1.26+ + Cobra (CLI commands), Bubble Tea + Bubbles + Lip Gloss (TUI), (001-build-profile-memory-cli)

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

- 002-portable-profiles: Added Go 1.26+ + Cobra (CLI), Bubble Tea/Bubbles/Lip Gloss (TUI)
- 001-build-profile-memory-cli: Added Go 1.26+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation under `internal/vector/hnsw` (no cgo)
- 001-build-profile-memory-cli: Added Go 1.26+ + Cobra, Bubble Tea, Lip Gloss, `modernc.org/sqlite`, OpenAI-compatible provider adapter, **new** pure-Go HNSW implementation (to be added under `internal/vector/hnsw` to avoid cgo)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
