# Implementation Plan: Bubble Tea TUI Standard

**Branch**: `004-bubbletea-tui` | **Date**: 2026-04-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-bubbletea-tui/spec.md`

## Summary

Refactor all existing TUI interaction flows to conform to the Bubble Tea application model, prefer Bubbles components where suitable, and define styling via Lip Gloss. New and existing TUI work must consistently follow Bubble Tea patterns and document any custom UI elements that replace potential Bubbles components.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Bubble Tea + Bubbles + Lip Gloss (TUI), Cobra (CLI)  
**Storage**: N/A (UI-only refactor)  
**Testing**: `go test` (unit + integration where applicable)  
**Target Platform**: Local terminal (macOS/Linux)  
**Project Type**: CLI/TUI application  
**Performance Goals**: TUI updates remain responsive (no perceptible lag during typical input/navigation).  
**Constraints**:
- All existing TUI surfaces must be migrated to Bubble Tea patterns.
- Prefer Bubbles components for any UI surface where a suitable component exists.
- Use Lip Gloss for styling definitions when styling is required.
- Document rationale for custom components when Bubbles is not suitable.

**Scale/Scope**: Single-terminal application; refactor all existing TUI flows in the repository.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

- **Code Quality Gate**: PASS
  - Refactor should keep modules focused and preserve explicit error handling.
  - Apply gofmt/goimports and existing lint rules to all touched files.
- **Testing Standards Gate**: PASS
  - Add/adjust tests for any refactored TUI flows to prevent regressions.
  - Include negative-path behavior where applicable (e.g., cancel, invalid input).
- **UX Consistency Gate**: PASS
  - Preserve existing CLI/TUI wording, navigation keys, and visual patterns unless explicitly updated.
  - Document any intentional UX deviations alongside the refactor.
- **Performance Gate**: PASS
  - Validate that refactored flows render without perceptible lag.
  - Capture at least one representative performance sanity check.

## Project Structure

### Documentation (this feature)

```text
specs/004-bubbletea-tui/
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
internal/
├── tui/                 # Bubble Tea models and views + Lip Gloss styles
├── commands/            # command registry
├── parser/              # slash input parsing
├── suggest/             # suggestion engine
├── chat/                # dispatcher and chat pipeline
└── app/                 # startup orchestration

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: Keep refactor changes within existing `internal/tui` flows and adjacent command/dispatch layers. Do not introduce new application layers.

## Complexity Tracking

No constitution violations anticipated.

## Phase 0: Outline & Research

### Research Tasks

1. Inventory all existing TUI flows and confirm Bubble Tea compliance status.
2. Identify which TUI surfaces can reuse existing Bubbles components and where custom UI is still required.
3. Identify styling definitions that should be consolidated into Lip Gloss styles.

### Research Output

- **Output**: `specs/004-bubbletea-tui/research.md`

## Phase 1: Design & Contracts

### Data Model

- Not applicable beyond documenting UI flow inventory and component usage.

**Output**: `specs/004-bubbletea-tui/data-model.md`

### Contracts

- Document the UI interaction contract for each refactored flow (keyboard navigation, inputs, outputs).

**Output**: `specs/004-bubbletea-tui/contracts/tui-flows.md`

### Quickstart

- Provide validation steps covering each refactored TUI flow (navigation, filtering, cancellation, error states).

**Output**: `specs/004-bubbletea-tui/quickstart.md`

### Agent Context Update

- Run `/home/andy/gitrepos/noto/.specify/scripts/bash/update-agent-context.sh pi`.

## Phase 2: Planning

- Phase 2 detailed task breakdown will be produced by `/speckit.tasks`.
