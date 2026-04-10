# Implementation Plan: Slash Command Navigation

**Branch**: `003-slash-command-navigation` | **Date**: 2026-04-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-slash-command-navigation/spec.md`

## Summary

Improve the existing slash command suggestion UX so users can efficiently discover, filter, navigate, and execute commands using only the keyboard. The key gap to address is that Up/Down navigation must traverse the *entire* suggestion list (not only the currently visible portion) by scrolling the suggestion window as needed.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (command registry is shared by CLI and slash execution)  
**Storage**: N/A (this feature is UI/interaction behavior; command registry already exists in-memory)  
**Testing**: `go test` (unit + integration where applicable)  
**Target Platform**: Local terminal (macOS/Linux)  
**Project Type**: CLI/TUI application  
**Performance Goals**: Slash suggestion refresh remains responsive during typing (budget: p95 < 50 ms per keystroke, consistent with existing project targets)  
**Constraints**:
- Preserve existing slash command registry and execution semantics
- Preserve existing input history navigation behavior when not in slash suggestion mode
- No regressions in existing picker overlays (model/profile/backup/extractor model)

**Scale/Scope**: Typical command registry size: tens to low hundreds of commands; suggestions may exceed visible terminal height.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

- **Code Quality Gate**: PASS
  - Keep suggestion UI logic small and testable.
  - Avoid duplicating complex list rendering logic (prefer reusing a single scrolling/windowing approach).
- **Testing Standards Gate**: PASS
  - Add automated tests for windowing/scroll behavior and key-handling edge cases (including boundary navigation and empty/no-match states).
- **UX Consistency Gate**: PASS
  - Follow existing interaction patterns used in the picker overlay (type to filter, ↑↓ to navigate, Enter to select).
  - Ensure slash suggestion mode does not break non-slash behavior (chat send, history navigation, viewport scroll).
- **Performance Gate**: PASS
  - Rendering and cursor movement must remain instantaneous for typical list sizes.
  - Add at least one targeted micro-benchmark or lightweight measurement plan for suggestion refresh/render.

## Project Structure

### Documentation (this feature)

```text
specs/003-slash-command-navigation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── slash-suggestions.md
└── tasks.md             # Produced by /speckit.tasks (not by this plan)
```

### Source Code (repository root)

```text
cmd/
internal/
├── tui/                 # suggestion rendering + key handling
├── suggest/              # suggestion ranking/filtering engine (existing)
├── chat/                 # dispatcher integrates slash + suggestions (existing)
├── commands/             # canonical command registry (existing)
└── parser/               # slash input parsing + prefix extraction (existing)

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: Keep this change localized to `internal/tui/model.go` (key handling + rendering) and small helper(s) for windowed rendering (either shared helper or reuse existing `pickerState` rendering patterns). Do not change slash parsing, registry, or command dispatch unless required for correctness.

## Complexity Tracking

No constitution violations anticipated.

## Phase 0: Outline & Research

### Research Tasks

1. **Assess current suggestion rendering behavior**
   - Locate the existing suggestion state and rendering in `internal/tui/model.go` (suggestion list, cursor handling, rendering).
   - Confirm the observed limitation: when the suggestions exceed available terminal height, selection can move but the user cannot see items beyond the visible window.

2. **Assess current keybinding behavior and conflicts**
   - Document current behavior for Up/Down when suggestions are visible vs. when they aren’t (history navigation, viewport scrolling).
   - Confirm how Tab and Enter are handled today in slash suggestion mode.

3. **Identify a consistent scrolling/windowing approach**
   - Compare picker overlay’s windowing logic (`internal/tui/picker.go`) to suggestion rendering.
   - Decide whether to:
     - reuse the picker windowing logic conceptually for suggestions, or
     - factor out a tiny shared “window” helper (start/end computation) used by both.

### Research Output

- **Output**: `specs/003-slash-command-navigation/research.md`

## Phase 1: Design & Contracts

### Design Goals

- Provide a suggestion list that:
  - supports filtering as the user types,
  - supports Up/Down traversing the entire list,
  - scrolls the suggestion window as needed so the selected item stays visible,
  - supports Tab to autofill the selected suggestion into input,
  - supports Enter to execute the selected command.

### Data Model

- Capture the minimal UI state needed for correct scrolling (cursor + windowing strategy).

**Output**: `specs/003-slash-command-navigation/data-model.md`

### Contracts

- Document the interaction contract for slash suggestions (keys, filtering semantics, selection, execution).

**Output**: `specs/003-slash-command-navigation/contracts/slash-suggestions.md`

### Quickstart

- Provide manual validation steps that exercise:
  - long suggestion lists,
  - scrolling across the full list,
  - filtering + scrolling,
  - Tab autofill,
  - Enter execution.

**Output**: `specs/003-slash-command-navigation/quickstart.md`

### Agent Context Update

- Run `/home/andy/gitrepos/noto/.specify/scripts/bash/update-agent-context.sh pi`.

## Phase 2: Planning

- Phase 2 detailed task breakdown will be produced by `/speckit.tasks`.
