# Implementation Plan: Settings Dialog Navigation (Profiles List)

**Branch**: `006-settings-dialog` | **Date**: 2026-04-12 | **Spec**: /home/andy/gitrepos/noto/specs/006-settings-dialog/spec.md
**Input**: Feature specification from `/specs/006-settings-dialog/spec.md`

## Summary

Implement profile management as a single settings list with Enter/Ctrl+N/Ctrl+R/Ctrl+D actions and remove profile slash commands, while keeping existing settings editor flows intact and using profile services for persistence.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: charm.land/bubbletea/v2, charm.land/bubbles/v2, charm.land/lipgloss/v2, Cobra  
**Storage**: profile.json + per-profile SQLite (modernc.org/sqlite)  
**Testing**: `go test ./...`, integration tests under tests/integration  
**Target Platform**: Terminal UI (Linux/macOS/Windows terminals)  
**Project Type**: TUI + CLI application  
**Performance Goals**: Settings dialog opens in <1s  
**Constraints**: No slash commands for profiles; must use existing profile service validation  
**Scale/Scope**: Single TUI settings flow

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: Use gofmt, golangci-lint (make lint), and standard Go conventions. Plan includes formatting + lint validation step.
- **Testing Standards Gate**: Add/adjust integration tests for profiles list actions and failure paths (delete last profile, rename validation).
- **UX Consistency Gate**: Use existing settings list/editor patterns; Ctrl+N/Ctrl+R/Ctrl+D in Profiles list; Enter selects and switches profile; Profiles list shows keybinding hints.
- **Performance Gate**: Manual check that settings opens in <1s with no lag (quickstart checklist).

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
├── commands/
├── profile/
├── store/
└── tui/

tests/
├── contract/
└── integration/
```

**Structure Decision**: Single Go module with internal packages and integration tests under tests/integration.

## Complexity Tracking

No constitution violations.
