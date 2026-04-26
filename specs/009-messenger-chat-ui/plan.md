# Implementation Plan: messenger-chat-ui

**Branch**: `009-messenger-chat-ui` | **Date**: 2026-04-26 | **Spec**: /home/andy/gitrepos/noto/specs/009-messenger-chat-ui/spec.md
**Input**: Feature specification from `/specs/009-messenger-chat-ui/spec.md`

## Summary

Implement profile-wide history scrolling in the TUI so users can scroll backward across all conversations in the active profile, with thin conversation boundary separators showing each conversation start date, while preserving zone-aware wheel behavior, global PageUp/PageDown message scrolling, and bounded input-history behavior.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: Cobra CLI, Bubble Tea v2, Bubbles v2 (textarea, viewport), Lip Gloss v2, modernc.org/sqlite
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for conversation/message data and input history
**Testing**: `go test` with unit/integration/contract suites
**Target Platform**: Desktop terminal (macOS/Linux/Windows)
**Project Type**: CLI + TUI application
**Performance Goals**: Maintain bounded startup by loading latest 10 messages; preserve smooth incremental back-scroll with anchor stability
**Constraints**: Profile isolation, non-fatal history errors, manual FR-012 baseline guard, input-history policy unchanged (3/3/12 + clear on send)
**Scale/Scope**: Single-user local profile with potentially large multi-conversation history

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Code Quality Is Enforced — ✅ planned quality gates (fmt/lint/vet/test)
- Testing Standards Are Non-Negotiable — ✅ unit/integration/contract coverage across primary and failure flows
- User Experience Consistency First — ✅ separators add context without altering baseline bubble alignment

Post-Phase-1 Re-check: ✅ no constitution violations introduced.

## Project Structure

### Documentation (this feature)

```text
specs/009-messenger-chat-ui/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── ui-scroll-behavior.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/noto/
internal/
├── app/
├── chat/
├── store/
└── tui/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Keep changes in current single-module Go project; extend store retrieval and TUI rendering/paging logic.

## Complexity Tracking

No constitution violations requiring justification.

## Phase 0: Research

Completed in `research.md`:
- Selected profile-wide multi-conversation history source.
- Selected separator rendering model at conversation boundaries.
- Selected unified chronological pagination strategy.
- Confirmed non-fatal paging error handling.
- Chosen deterministic Go local-time formatting for separator labels (`time.Local`, layout `2006-01-02 15:04 MST`).

## Phase 1: Design & Contracts

Outputs generated:
- `data-model.md` defining `ProfileHistoryWindow`, `HistoryItem`, `ConversationBoundaryItem`, and input-history window rules.
- `contracts/ui-scroll-behavior.md` documenting profile-wide paging, separator rendering, and routing guarantees.
- `quickstart.md` covering cross-conversation validation steps.

## Phase 2: Implementation Plan

1. Extend store queries to fetch profile-wide ordered messages and carry conversation metadata needed for boundary rendering.
2. Update startup loading to source last 10 messages from profile-wide history, not only one conversation.
3. Update lazy-load cursoring to continue across older conversations in fixed batches of 10.
4. Add boundary item synthesis when conversation ID changes in loaded stream.
5. Render thin boundary separators with conversation start date using Go local time (`time.Local`) and fixed layout `2006-01-02 15:04 MST`.
6. Preserve viewport anchor during prepend operations including boundary insertions.
7. Keep wheel routing zone-aware and PageUp/PageDown global to messages history.
8. Keep input-history policy unchanged (no preload, 3/3/12, clear on send).
9. Add/adjust tests for cross-conversation loading, boundary rendering, no-op at absolute top, and non-fatal failures.
10. Run quality gates (`make fmt && make lint && make vet && make test`).
