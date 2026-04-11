# noto Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-04-11

## Active Technologies
- Go 1.26+ + Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (command registry is shared by CLI and slash execution) (003-slash-command-navigation)
- N/A (this feature is UI/interaction behavior; command registry already exists in-memory) (003-slash-command-navigation)
- Go 1.26+ + Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (CLI) (004-bubbletea-tui)
- N/A (UI-only refactor) (004-bubbletea-tui)
- Go 1.26+ + charm.land/bubbletea/v2, charm.land/bubbles/v2, charm.land/lipgloss/v2, Cobra (006-settings-dialog)
- Profile metadata (profile.json) + per-profile SQLite for existing provider/system prompt data (006-settings-dialog)

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
- 006-settings-dialog: Added Go 1.26+ + charm.land/bubbletea/v2, charm.land/bubbles/v2, charm.land/lipgloss/v2, Cobra
- 005-memory-context: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]
- 004-bubbletea-tui: Added Go 1.26+ + Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (CLI)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
