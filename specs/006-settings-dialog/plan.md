# Implementation Plan: Settings Dialog Navigation – Profile Management

**Branch**: `006-settings-dialog` | **Date**: 2026-04-12 | **Spec**: /home/andy/gitrepos/noto/specs/006-settings-dialog/spec.md
**Input**: Feature specification from `/specs/006-settings-dialog/spec.md`

## Summary

Add profile management user stories and tests to the settings dialog. Extend the settings menu with a Profiles submenu that dispatches to existing profile services/commands (select/create/rename/delete). Add integration tests to validate submenu navigation, action selection, prompts, confirmation, and UI updates.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Bubble Tea v2, Bubbles v2, Lip Gloss v2, Cobra  
**Storage**: profile.json + per-profile SQLite (`~/.noto/profiles/<slug>/memory.db`)  
**Testing**: `go test ./...`, integration tests under `tests/integration`  
**Target Platform**: Local CLI/TUI (terminal)  
**Project Type**: CLI/TUI application  
**Performance Goals**: Settings dialog opens in <1s; profile actions feel instantaneous  
**Constraints**: Offline/local-only; no backward compatibility required  
**Scale/Scope**: Single-user profile management, small local datasets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: All changes must pass `gofmt`, `go test ./...`, and `make lint`. Use explicit error handling and existing TUI patterns.
- **Testing Standards Gate**: Add integration tests for profile-management actions (select/create/rename/delete) and failure paths/confirmation behavior.
- **UX Consistency Gate**: Reuse settings dialog list + Enter/Esc interaction and existing profile prompt/confirmation patterns.
- **Performance Gate**: Verify settings and profile submenu open in <1s; record manual check in quickstart.

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
cmd/
└── noto/
internal/
├── app/
├── chat/
├── commands/
├── config/
├── profile/
├── store/
└── tui/

tests/
├── contract/
└── integration/
```

**Structure Decision**: Use existing CLI/TUI structure with settings work in `internal/tui`, profile services in `internal/profile`, and integration tests in `tests/integration`.

## Complexity Tracking

None.

---

## Phase 0: Outline & Research

### Unknowns

None. Existing profile service and command patterns are already in the codebase.

### Research Tasks

- Confirm existing profile commands (select/create/rename/delete) and expected TUI entry points.
- Review profile metadata shape and validation rules for menu prompts and confirmation flows.

### Output

- `research.md`

---

## Phase 1: Design & Contracts

### Data Model

- Document profile action entries and mapping to existing profile services.

### Contracts

- No external contracts; note in `contracts/README.md`.

### Quickstart

- Document profile submenu navigation and action flows.
- Add manual performance check (<1s open).

### Agent Context Update

- Run `.specify/scripts/bash/update-agent-context.sh pi`.

### Output

- `data-model.md`
- `contracts/README.md`
- `quickstart.md`

---

## Phase 2: Planning (stop after this phase)

- Add user stories + tests for profile management settings flow.
- Generate tasks in `tasks.md` after design is complete.
