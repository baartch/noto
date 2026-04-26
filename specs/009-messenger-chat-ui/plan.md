# Implementation Plan: messenger-chat-ui

**Branch**: `009-messenger-chat-ui` | **Date**: 2026-04-26 | **Spec**: /home/andy/gitrepos/noto/specs/009-messenger-chat-ui/spec.md
**Input**: Feature specification from `/specs/009-messenger-chat-ui/spec.md`

## Summary

Implement messenger-style history continuity and scroll behavior in the TUI by loading the latest conversation messages on startup, routing mouse wheel scrolls by cursor zone (messages vs input) while Page Up/Page Down always scroll message history, adding bounded lazy loading for older conversation and input-history entries, and resetting transient input-history memory after send.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: Cobra CLI, Bubble Tea v2, Bubbles v2 (textarea, viewport), Lip Gloss v2, modernc.org/sqlite
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for conversations/messages and persisted input history
**Testing**: `go test` unit/integration/contract suites (`tests/unit`, `tests/integration`, `tests/contract`)
**Target Platform**: Desktop terminal environments (macOS/Linux/Windows)
**Project Type**: CLI + TUI application
**Performance Goals**: Startup render under 2s with only latest 10 conversation messages loaded; smooth incremental history scrolling without visible jump
**Constraints**: Strict UX zone isolation for wheel events; input-history in-memory cap 12 with no preload; preserve existing profile isolation and non-fatal error handling
**Scale/Scope**: Single-user local profiles, potentially long conversation histories (50+ messages), bounded in-memory windows for history browsing

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Code Quality Is Enforced — ✅ Plan includes focused changes in TUI/store layers and quality gates (`fmt`, `lint`, `vet`).
- Testing Standards Are Non-Negotiable — ✅ Plan includes unit/integration/contract coverage for startup load, zone routing, lazy loading, and failure paths.
- User Experience Consistency First — ✅ Plan preserves existing messenger bubble patterns and enforces deterministic scroll-zone behavior.

Post-Phase-1 Re-check: ✅ No violations introduced by research/design artifacts.

Manual FR-012 Baseline Verification (2026-04-26): ✅ Reviewer sign-off recorded — user bubbles remain right-aligned, assistant bubbles remain left-aligned; no layout changes introduced by this feature.

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
└── tasks.md             # Created later by /speckit.tasks
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

**Structure Decision**: Keep a single Go module and implement feature changes primarily in `internal/tui` for behavior/render flow and `internal/store`/`internal/chat` for bounded history retrieval.

## Complexity Tracking

No constitution violations requiring justification.

## Phase 0: Research

Completed in `research.md`:
- Chosen strict cursor-zone wheel routing over focus/overflow alternatives.
- Confirmed startup/lazy load conversation policy (10 initial, 10 batch).
- Incorporated clarified input-history policy (0 preload, 3 first-load, 3 batch, max 12, clear after send).
- Selected stable-anchor prepend behavior for conversation lazy loading.
- Chosen non-fatal history-read error handling.

## Phase 1: Design & Contracts

Completed artifacts:
- `data-model.md`: Defines `ConversationHistoryWindow`, `InputHistoryWindow`, `ScrollZone`, `ChatMessageView` with validation/state transitions.
- `contracts/ui-scroll-behavior.md`: Defines UI-observable contract for routing, loading, reset, and failure handling.
- `quickstart.md`: Manual validation path and test/quality commands.

## Phase 2: Implementation Plan

1. **Conversation history retrieval support**
   - Add store-layer queries for latest N messages and older-than cursor pagination for a conversation.
   - Ensure retrieval remains profile-constrained through conversation selection.

2. **Startup continuity wiring**
   - On TUI initialization/profile switch, load latest 10 conversation messages.
   - Map stored messages to chat view roles and open viewport at bottom.

3. **Zone-aware wheel routing + page-key routing**
   - Detect cursor hover zone (`messages` vs `input`) from mouse events.
   - Route mouse wheel events exclusively to the selected zone; remove overflow handoff behavior.
   - Route Page Up/Page Down to messages history scrolling regardless of hover zone.

4. **Conversation lazy loading**
   - Trigger older-message fetch at messages-view top boundary using batch size 10.
   - Prepend new messages while preserving visual anchor to avoid jumps.

5. **Input-history lazy loading and cap**
   - Keep input-history cache empty until first input-zone scroll.
   - Load 3 on first request, then older in +3 increments up to max 12.
   - Maintain draft restoration semantics for forward navigation.

6. **Send-time input-history reset**
   - After successful send, clear in-memory input-history window only.
   - Keep persisted input history unchanged.

7. **Error handling and UX messaging**
   - Treat history retrieval failures as non-fatal.
   - Surface inline error feedback without blocking interaction.

8. **Tests and regression coverage**
   - Unit tests: scroll-zone routing, pagination cursor logic, cap enforcement, send reset.
   - Integration tests: startup load, profile switch reload, conversation anchor preservation, failure behavior.
   - Contract/UX tests: wheel isolation between zones and lazy-load invariants.

9. **Quality gates before merge**
   - Run `make fmt`, `make lint`, `make vet`, `make test`.
   - Status (2026-04-26): ✅ passed (`make fmt && make lint && make vet && make test`).
