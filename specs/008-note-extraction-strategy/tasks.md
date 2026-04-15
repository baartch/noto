# Tasks: note-extraction-strategy

**Input**: Design documents from `/specs/008-note-extraction-strategy/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Confirm existing provider_config schema usage in internal/store/migrations/profile/*.sql
- [X] T002 [P] Review embedding model persistence in internal/profile/settings.go
- [X] T003 [P] Review settings UI bindings in internal/tui/model.go

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T004 Add provider_config embeddings_model column migration in internal/store/migrations/profile/
- [X] T005 Remove unused columns from new profile schema in internal/store/migrations/profile/ (no drop migration needed)

---

## Phase 3: Foundational (2b) - Provider config migration stability

**Goal**: Ensure provider config changes do not regress note capture.
**Independent Test**: Run a chat session and verify no duplicate notes are created.

### Tests for migration stability

- [X] T006 [P] Add regression test for duplicate note detection in tests/integration/

### Validation for migration stability

- [X] T007 Validate note capture flow still passes after provider_config migration in internal/memory/ (update if needed)

---

## Phase 4: User Story 2 - Surface important notes during chat (Priority: P2)

**Goal**: Ensure embeddings model persistence is used in retrieval context.
**Independent Test**: Change embeddings model and confirm retrieval uses updated config after restart.

### Tests for User Story 2

- [X] T008 [P] [US2] Add integration test for embeddings model retrieval flow in tests/integration/

### Implementation for User Story 2

- [X] T009 [US2] Update provider_config usage for embeddings model in internal/app/chat_cmd.go
- [X] T010 [US2] Wire embeddings model into vector indexing in internal/vector/sync.go (Syncer) and internal/vector/index.go (manifest header)

---

## Phase 5: User Story 3 - Review and manage captured notes (Priority: P3)

**Goal**: Settings list updates instantly when embeddings model changes.
**Independent Test**: Change embeddings model in Settings and verify list updates immediately.

### Tests for User Story 3

- [X] T011 [P] [US3] Add UI regression test for settings list refresh in tests/integration/

### Implementation for User Story 3

- [X] T012 [P] [US3] Extend store.ProviderConfig with embeddings model in internal/store/provider_config_repo.go
- [X] T013 [US3] Update provider_config upsert/select to include embeddings_model in internal/store/provider_config_repo.go
- [X] T014 [US3] Migrate profile embedding model to provider_config in internal/profile/settings.go
- [X] T015 [US3] Load embeddings model from provider_config in internal/tui/model.go
- [X] T016 [US3] Persist embeddings model selection via provider_config in internal/tui/model.go
- [X] T017 [US3] Refresh settings list immediately after embeddings model picker selection in internal/tui/model.go
- [X] T018 [US3] Add UX validation checklist for settings refresh in specs/008-note-extraction-strategy/quickstart.md

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T019 [P] Run quickstart validation steps in specs/008-note-extraction-strategy/quickstart.md
- [ ] T020 [P] Add regression coverage for provider_config embeddings model in tests/integration/
- [ ] T021 [P] Run gofmt/go vet/go test/lint for code quality gates
- [ ] T022 Update docs if provider_config schema changes are user-facing in docs/

---

## Dependencies & Execution Order

- Setup (Phase 1) → Foundational (Phase 2) → User Stories (Phases 3-5) → Polish (Phase 6)
- User stories can proceed after Phase 2, with P1 recommended before P2/P3.

## Parallel Execution Examples

- Phase 1: T002 and T003 can run in parallel.
- Phase 3: T006 can run in parallel with other story tests.
- Phase 5: T012 and T013 can run in parallel.
- Phase 6: T019 and T020 can run in parallel.

## Implementation Strategy

- MVP: Complete Phases 1-3 and validate quickstart.
- Incremental: Add P2 and P3 flows, then cross-cutting cleanup.
