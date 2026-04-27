# Tasks: Bubble Tea TUI Standard

**Input**: Design documents from `/specs/004-bubbletea-tui/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are required by the specification (NFR-002), so this task list includes test tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- All tasks include exact file paths

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish implementation scaffolding for TUI telemetry and validation.

- [ ] T001 Audit current TUI flows and capture inventory in `specs/004-bubbletea-tui/contracts/tui-flows.md`
- [ ] T002 Add/confirm test fixtures for provider usage payloads in `tests/unit/provider/usage_payload_fixtures_test.go`
- [ ] T003 [P] Add reusable footer rendering test helper in `tests/integration/tui/footer_test_helpers.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core telemetry and footer primitives that all story work depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Implement usage snapshot parsing types and mappers in `internal/provider/usage.go`
- [ ] T005 [P] Implement session usage accumulator (`up/down/cache read/cache write/cost`) in `internal/tui/usage_accumulator.go`
- [ ] T006 [P] Implement accumulator update logic for `main|extractor|embeddings` sources in `internal/tui/usage_accumulator.go`
- [ ] T007 Implement footer status view model with required always-visible fields in `internal/tui/footer_status.go`
- [ ] T008 Wire provider usage events into TUI model update path in `internal/tui/model.go`
- [ ] T009 Add unit tests for parsing + missing-usage no-op behavior in `tests/unit/provider/usage_test.go`
- [ ] T010 Add unit tests for accumulator math and source aggregation in `tests/unit/tui/usage_accumulator_test.go`

**Checkpoint**: Foundation ready - user story implementation can now begin.

---

## Phase 3: User Story 1 - Consistent Bubble Tea TUI Usage (Priority: P1) 🎯 MVP

**Goal**: Ensure all TUI flows follow Bubble Tea patterns, keybindings, anchored layout, and footer telemetry behavior.

**Independent Test**: Verify all TUI entry points use Bubble Tea models/update loops; verify overlay anchoring, keybindings, help placement, and required footer fields.

### Tests for User Story 1

- [ ] T011 [P] [US1] Add integration test for anchored input/footer during overlays in `tests/integration/tui/layout_anchor_test.go`
- [ ] T012 [P] [US1] Add integration test for picker `/` filtering without list collapse in `tests/integration/tui/picker_filter_test.go`
- [ ] T013 [P] [US1] Add integration test for keybindings (`Ctrl+D`, `Ctrl+L`, `Ctrl+H`, `Ctrl+J`) in `tests/integration/tui/keybindings_test.go`
- [ ] T014 [P] [US1] Add integration test for expanded help position above textarea in `tests/integration/tui/help_layout_test.go`
- [ ] T015 [P] [US1] Add integration test for always-visible footer fields and `ctx:miss|hit` in `tests/integration/tui/footer_fields_test.go`
- [ ] T016 [P] [US1] Add integration test for footer usage accumulation across main/extractor/embeddings in `tests/integration/tui/footer_usage_aggregation_test.go`

### Implementation for User Story 1

- [ ] T017 [US1] Refactor remaining non-conforming TUI flows to Bubble Tea model/update loop patterns in `internal/tui/model.go`
- [ ] T018 [US1] Enforce overlay layout anchoring for input/footer in `internal/tui/layout.go`
- [ ] T019 [US1] Implement stable picker filter behavior with Bubbles list filtering in `internal/tui/pickers.go`
- [ ] T020 [US1] Implement/normalize global keybindings (`Ctrl+D`, `Ctrl+L`, `Ctrl+H`, `Ctrl+J`) in `internal/tui/keys.go`
- [ ] T021 [US1] Render expanded help above textarea and preserve footer placement in `internal/tui/help.go`
- [ ] T022 [US1] Render footer telemetry fields (`up/down/cache read/cache write/cost`, `ctx:miss|hit`, profile/model/version/help) in `internal/tui/footer.go`
- [ ] T023 [US1] Wire usage payload handling from provider responses/chunks into footer updates in `internal/tui/model.go`

**Checkpoint**: User Story 1 is independently functional and testable (MVP).

---

## Phase 4: User Story 2 - Prefer Bubbles Components (Priority: P2)

**Goal**: Ensure existing Bubbles components are used whenever suitable and custom components are justified.

**Independent Test**: Inspect each TUI surface and verify Bubbles usage where applicable; any custom UI has documented rationale.

### Tests for User Story 2

- [ ] T024 [P] [US2] Add contract test validating flow/component mapping and custom-rationale requirement in `tests/contract/tui_component_contract_test.go`
- [ ] T025 [P] [US2] Add integration regression test for Bubbles Help primary/secondary key grouping in `tests/integration/tui/help_component_grouping_test.go`

### Implementation for User Story 2

- [ ] T026 [US2] Replace remaining custom list/input/help primitives with Bubbles components where suitable in `internal/tui/components.go`
- [ ] T027 [US2] Document rationale for retained custom components in `specs/004-bubbletea-tui/contracts/tui-flows.md`
- [ ] T028 [US2] Consolidate shared Lip Gloss style definitions for standardized TUI surfaces in `internal/tui/styles.go`

**Checkpoint**: User Stories 1 and 2 both work independently.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, cleanup, and documentation.

- [ ] T029 [P] Update quick verification steps for telemetry and keybinding behavior in `specs/004-bubbletea-tui/quickstart.md`
- [ ] T030 Run full quality gates and fix residual issues (`make fmt && make lint && make vet && make test`) in repository root

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies.
- **Phase 2 (Foundational)**: Depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2.
- **Phase 4 (US2)**: Depends on Phase 2; may run after or alongside late US1 tasks, but final merge should follow US1 completion.
- **Phase 5 (Polish)**: Depends on completion of selected user stories (US1 required, US2 recommended).

### User Story Dependencies

- **US1 (P1)**: No dependency on other stories after Foundational phase.
- **US2 (P2)**: Depends on shared foundations; should remain independently testable.

### Within Each User Story

- Tests first, then implementation.
- Layout/state model updates before rendering polish.
- Story must pass its independent test criteria before moving on.

---

## Parallel Execution Examples

### User Story 1

```bash
# Parallel test authoring
T011, T012, T013, T014, T015, T016

# Parallel implementation chunks after tests are in place
T018, T019, T020, T021, T022
```

### User Story 2

```bash
# Parallel validation tasks
T024, T025

# Parallel implementation/documentation tasks
T027, T028
```

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1 and Phase 2.
2. Complete all US1 tests and implementation tasks.
3. Validate US1 independently via integration tests and quickstart flow.

### Incremental Delivery

1. Deliver US1 as MVP.
2. Deliver US2 as component-standardization increment.
3. Execute polish tasks and run full quality gates.

### Parallel Team Strategy

1. Team completes Setup + Foundational.
2. Split US1 work: layout/keybindings/help/footer/aggregation in parallel by file boundaries.
3. Assign US2 to a second track once foundations stabilize.
