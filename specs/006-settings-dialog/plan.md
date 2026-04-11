# Implementation Plan: Settings Dialog Navigation

**Branch**: `006-settings-dialog` | **Date**: 2026-04-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/006-settings-dialog/spec.md`

## Summary

Add a Ctrl+, settings dialog built with Bubble Tea + Bubbles list, showing alphabetically sorted key/value entries and submenus. Support editing values via a textarea editor (Enter saves, Esc cancels) with numeric validation, submenu navigation with Esc to go up, and top-level Esc to close. Cover all app settings (model/extractor model, provider configuration submenu, token budget, system prompt edit) while keeping UX consistent with existing TUI patterns and storing system prompt in the profile database.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: charm.land/bubbletea/v2, charm.land/bubbles/v2, charm.land/lipgloss/v2, Cobra
**Storage**: Profile metadata (profile.json) + per-profile SQLite for existing provider data/system prompt/data
**Testing**: `go test ./...`, integration tests under `tests/integration`
**Target Platform**: Terminal (Linux/macOS/Windows)
**Project Type**: CLI/TUI application
**Performance Goals**: Settings dialog opens in under 1 second; edits apply immediately
**Constraints**: Ctrl+, opens settings; Esc navigates up or closes; alphabetical ordering; Enter saves, Esc cancels
**Scale/Scope**: Single-profile settings dialog covering all current app settings

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

- **Code Quality Gate**: Run `golangci-lint run` (via `make lint`) and `go test ./...` before merge.
- **Testing Standards Gate**: Add integration coverage for Ctrl+, dialog navigation, value editing, and Esc behavior.
- **UX Consistency Gate**: Match existing footer/help patterns and picker overlay layout; document deviations if needed.
- **Performance Gate**: Confirm settings dialog opens in under 1 second for typical profiles.

## Project Structure

### Documentation (this feature)

```text
specs/006-settings-dialog/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md
```

### Source Code (repository root)

```text
internal/
├── tui/
│   ├── model.go
│   └── ...
├── commands/
├── profile/
├── store/
└── app/

tests/
├── integration/
└── contract/
```

**Structure Decision**: Single Go CLI/TUI project; settings dialog lives under `internal/tui` with command/state wiring in `internal/app` and persistence in `internal/profile`/`internal/store`.

## Phase 0: Research

### Research Tasks

- Confirm best-practice UX for nested settings dialogs in Bubble Tea.
- Confirm Bubbles list + textarea usage for editable settings and navigation keys.

### Output

- `research.md` with decisions and rationale.

## Phase 1: Design & Contracts

### Data Model

Define logical settings entities (menu, entry, value) and how they map to existing profile/provider data, including system prompt stored in SQLite.

### Contracts

No external API contracts required (internal UI flow only).

### Quickstart

Document manual validation steps for opening settings, navigating submenus, editing values, and ensuring ordering.

### Agent Context Update

Run: `.specify/scripts/bash/update-agent-context.sh pi`

## Phase 2: Implementation Plan

1. Add Ctrl+, keybinding to open settings dialog and surface in footer help.
2. Build settings dialog list with alphabetical ordering, value entries, and submenu entries.
3. Implement submenu navigation (Enter to enter, Esc to go back) and top-level Esc to close.
4. Implement value editor using Bubbles textarea; Enter saves, Esc cancels; validate numeric entries with inline error feedback.
5. Wire settings entries to current settings sources: model/extractor model, provider configuration submenu, token budget, system prompt stored in SQLite.
6. Add integration tests for open/close, submenu navigation, edit/save/cancel flows, and numeric validation errors.

## Complexity Tracking

> No constitution violations.
