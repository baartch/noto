# Research: Profile Management in Settings

## Decision
Reuse existing profile commands and services (select/create/rename/delete) from `internal/commands` and `internal/profile` for settings actions.

## Rationale
- Keeps CLI and settings dialog consistent.
- Avoids duplicating validation and filesystem/db update logic.

## Alternatives Considered
- **New settings-specific profile service**: rejected; duplicates existing functionality.
- **Direct DB writes from TUI**: rejected; bypasses confirmation and validation rules.
