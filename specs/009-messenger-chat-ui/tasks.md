# Tasks: Messenger Chat UI History Scrolling (Cross-Conversation)

**Input**: Design documents from `/specs/009-messenger-chat-ui/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ui-scroll-behavior.md, quickstart.md

**Tests**: Required by constitution and plan; include unit/integration/contract coverage before implementation where applicable.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task can run in parallel (separate files/no blocking dependency)
- **[Story]**: User story label (`[US1]`, `[US2]`, `[US3]`) for story-phase tasks
- Every task includes explicit file path

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare constants/types/render helpers for profile-wide history stream.

- [X] T001 Create/update shared history constants for startup/lazy-load/input policy in `internal/tui/constants.go`
- [X] T002 [P] Add conversation boundary view helper (thin divider + date label) in `internal/tui/bubble.go`
- [X] T003 [P] Add/confirm history item typing scaffolding (`message` vs `boundary`) in `internal/tui/model.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data access and mapping needed by all user stories.

**⚠️ CRITICAL**: Complete before user-story work.

- [X] T004 Implement profile-wide recent-message query across conversations (ordered by message time) in `internal/store/message_repo.go`
- [X] T005 [P] Implement profile-wide older-than-cursor paging query across conversations in `internal/store/message_repo.go`
- [X] T006 Implement conversation metadata fetch helper (for boundary start-date labels) in `internal/store/conversation_repo.go`
- [X] T007 [P] Add/adjust mapping utility from store messages to TUI history items in `internal/tui/model.go`
- [X] T008 Add shared non-fatal history loading error channel/state for paging failures in `internal/tui/model.go`

**Checkpoint**: Foundational layer ready.

---

## Phase 3: User Story 1 - Conversation Continuity on Startup (Priority: P1) 🎯 MVP

**Goal**: Startup shows latest profile-wide history context (not limited to single conversation), bottom-positioned and resilient to failures.

**Independent Test**: Open app with multi-conversation profile and verify latest 10 messages appear at bottom; empty/failure remains usable.

### Tests for User Story 1

- [X] T009 [P] [US1] Add unit test for startup profile-wide latest-window selection in `tests/unit/tui/history_window_test.go`
- [X] T010 [P] [US1] Add integration test for startup loading latest 10 across conversations in `tests/integration/tui_startup_history_test.go`
- [X] T011 [P] [US1] Add integration test for profile-switch startup reload using new profile-wide source in `tests/integration/tui_profile_switch_history_test.go`
- [X] T012 [P] [US1] Add integration test for non-fatal startup load failure behavior in `tests/integration/tui_history_failure_test.go`

### Implementation for User Story 1

- [X] T013 [US1] Wire startup history source to profile-wide query in `internal/app/chat_cmd.go`
- [X] T014 [US1] Apply startup history window and bottom positioning in `internal/tui/model.go`
- [X] T015 [US1] Keep empty-state rendering non-blocking in `internal/tui/model.go`
- [X] T016 [US1] Surface startup history read failures as non-fatal inline feedback in `internal/tui/model.go`

**Checkpoint**: US1 functional and testable independently.

---

## Phase 4: User Story 2 - Zone-Aware Scrolling + Input History Policy (Priority: P2)

**Goal**: Preserve strict wheel zone isolation and global Page key behavior while keeping input-history 3/3/12 policy.

**Independent Test**: Wheel in messages/input affects only respective zone; PageUp/PageDown always messages; input-history policy preserved and reset after send.

### Tests for User Story 2

- [X] T017 [P] [US2] Add/refresh unit tests for scroll-zone routing decisions in `tests/unit/tui/scroll_zone_routing_test.go`
- [X] T018 [P] [US2] Add/refresh unit tests for input-history lazy-load policy (no preload, 3/3/12) in `tests/unit/tui/input_history_window_test.go`
- [X] T019 [P] [US2] Add integration test for wheel isolation (messages zone) in `tests/integration/tui_scroll_zone_isolation_test.go`
- [X] T020 [P] [US2] Add integration test for wheel isolation (input zone) in `tests/integration/tui_scroll_zone_isolation_test.go`
- [X] T021 [P] [US2] Add integration test for global PageUp/PageDown routing in `tests/integration/tui_pagekey_history_routing_test.go`
- [X] T022 [P] [US2] Add integration test for in-memory input-history reset after send in `tests/integration/tui_input_history_reset_test.go`
- [X] T023 [P] [US2] Add contract test for input-history policy invariants in `tests/contract/tui_input_history_contract_test.go`

### Implementation for User Story 2

- [X] T024 [US2] Ensure wheel routing remains zone-bound (`messages`/`input`) in `internal/tui/model.go`
- [X] T025 [US2] Ensure PageUp/PageDown route to messages history regardless of hover zone in `internal/tui/model.go`
- [X] T026 [US2] Preserve no-overflow-handoff behavior between input and messages in `internal/tui/model.go`
- [X] T027 [US2] Preserve on-demand input-history first-load (3) in `internal/tui/model.go`
- [X] T028 [US2] Preserve input-history +3 lazy-load and cap=12 in `internal/tui/model.go`
- [X] T029 [US2] Preserve draft restoration semantics for forward navigation in `internal/tui/model.go`
- [X] T030 [US2] Preserve reset of in-memory input-history window after send in `internal/tui/model.go`

**Checkpoint**: US1 and US2 independently functional.

---

## Phase 5: User Story 3 - Cross-Conversation Lazy Loading + Boundaries (Priority: P3)

**Goal**: Scroll backward through all profile conversations with boundary separators and stable viewport anchor.

**Independent Test**: With multi-conversation history, repeated top-boundary scrolling prepends older batches across conversations, shows separators with start dates, and no-ops at absolute top.

### Tests for User Story 3

- [X] T031 [P] [US3] Add unit tests for cross-conversation paging cursor/has-older transitions in `tests/unit/tui/conversation_paging_test.go`
- [X] T032 [P] [US3] Add integration test for prepend across conversation boundaries in `tests/integration/tui_conversation_lazyload_test.go`
- [X] T033 [P] [US3] Add integration test for boundary separator rendering with start date label in `tests/integration/tui_conversation_lazyload_test.go`
- [X] T033a [P] [US3] Add unit test for separator format (`-- YYYY-MM-DD HH:MM MST -----` width-expanding) using Go local time (`time.Local`) with layout `2006-01-02 15:04 MST` in `tests/unit/tui/conversation_boundary_format_test.go`
- [X] T034 [P] [US3] Add integration test for viewport anchor preservation during prepend in `tests/integration/tui_conversation_anchor_test.go`
- [X] T035 [P] [US3] Add integration test for no-op at absolute top across all profile conversations in `tests/integration/tui_conversation_lazyload_test.go`
- [X] T036 [P] [US3] Add integration test for non-fatal error during older-batch load in `tests/integration/tui_history_failure_test.go`
- [X] T037 [P] [US3] Add contract test for cross-conversation scroll behavior + separators in `tests/contract/tui_conversation_scroll_contract_test.go`

### Implementation for User Story 3

- [X] T038 [US3] Implement profile-wide older-batch fetch trigger at messages top boundary in `internal/tui/model.go`
- [X] T039 [US3] Integrate cross-conversation paging query flow in `internal/app/chat_cmd.go`
- [X] T040 [US3] Insert synthetic boundary items when conversation ID changes in loaded stream in `internal/tui/model.go`
- [X] T041 [US3] Render single-line conversation boundary `-- YYYY-MM-DD HH:MM MST -----` using Go local time (`time.Local`) with layout `2006-01-02 15:04 MST`, with right-side dashes expanding to viewport width in `internal/tui/bubble.go`
- [X] T042 [US3] Preserve viewport anchor when prepending messages + boundaries in `internal/tui/model.go`
- [X] T043 [US3] Stop further fetches when absolute profile-history top reached in `internal/tui/model.go`
- [X] T044 [US3] Keep send/reply bottom-snap behavior after mid-history browsing in `internal/tui/model.go`

**Checkpoint**: All user stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final docs, manual UX guard, and quality gates.

- [X] T045 [P] Update validation guidance for cross-conversation boundaries in `specs/009-messenger-chat-ui/quickstart.md`
- [X] T046 [P] Refresh contract wording for separators + profile-wide paging in `specs/009-messenger-chat-ui/contracts/ui-scroll-behavior.md`
- [X] T047 Run full quality gates (`make fmt && make lint && make vet && make test`) and record result in `specs/009-messenger-chat-ui/plan.md`
- [X] T048 Manually verify FR-012 baseline alignment unchanged and record reviewer sign-off in `specs/009-messenger-chat-ui/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1: no dependencies
- Phase 2: depends on Phase 1; blocks all user stories
- Phase 3 (US1): depends on Phase 2
- Phase 4 (US2): depends on Phase 2 (can proceed alongside US1 with coordination)
- Phase 5 (US3): depends on Phase 2 and US1 startup/history base
- Phase 6: depends on all desired story completion

### User Story Dependencies

- **US1 (P1)**: foundation only
- **US2 (P2)**: foundation only (shares `internal/tui/model.go` heavily)
- **US3 (P3)**: relies on US1 history source and foundational paging primitives

### Dependency Graph

- Foundation: `T001 -> {T004,T005,T006} -> {T007,T008}`
- US1: `{T009,T010,T011,T012} -> {T013,T014,T015,T016}`
- US2: `{T017,T018,T019,T020,T021,T022,T023} -> {T024,T025,T026,T027,T028,T029,T030}`
- US3: `{T031,T032,T033,T033a,T034,T035,T036,T037} -> {T038,T039,T040,T041,T042,T043,T044}`
- Polish: `{T045,T046,T047,T048}`

---

## Parallel Execution Examples

### User Story 1

```bash
# Parallel US1 test tasks
T009  T010  T011  T012

# Then implementation sequence
T013 -> T014 -> T015 -> T016
```

### User Story 2

```bash
# Parallel US2 test tasks
T017  T018  T019  T020  T021  T022  T023

# Then implementation in staged groups
T024/T025/T026
T027/T028/T029
Then T030
```

### User Story 3

```bash
# Parallel US3 test tasks
T031  T032  T033  T033a  T034  T035  T036  T037

# Then cross-conversation implementation sequence
T038 -> T039 -> T040 -> T041 -> T042 -> T043 -> T044
```

---

## Implementation Strategy

### MVP First

1. Complete Phases 1–2.
2. Deliver US1 (Phase 3) for startup continuity with profile-wide source.
3. Validate US1 tests and demo.

### Incremental Delivery

1. Add US2 to preserve input-zone behavior and global page-key routing.
2. Add US3 for cross-conversation lazy loading + separators.
3. Finish polish and gate commands.

### Parallel Team Strategy

1. Coordinate edits in shared files (`internal/tui/model.go`, `internal/app/chat_cmd.go`).
2. Split work by layer:
   - Engineer A: store queries + metadata
   - Engineer B: TUI rendering/routing
   - Engineer C: tests/contracts/docs
3. Rebase frequently due to shared TUI file.

---

## Notes

- All tasks follow required checklist format with IDs, labels, and file paths.
- Tests included per constitution and plan requirements.
- Cross-conversation separators are a UX addition; FR-012 bubble alignment remains unchanged and manually verified in T048.
