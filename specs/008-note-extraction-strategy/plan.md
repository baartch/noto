# Implementation Plan: note-extraction-strategy

**Branch**: `008-note-extraction-strategy` | **Date**: 2026-04-15 | **Spec**: /home/andy/gitrepos/noto/specs/008-note-extraction-strategy/spec.md
**Input**: Feature specification from `/specs/008-note-extraction-strategy/spec.md`

## Summary

Persist the selected embeddings model in the profile database (provider_config), remove unused provider_config columns, and ensure the Settings list reflects updated model selections immediately after a change.

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: Cobra, Bubble Tea v2, Bubbles v2, Lip Gloss v2, modernc.org/sqlite
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) and vector index (`~/.noto/profiles/<profile>/memory.vec`)
**Testing**: `go test` (tests/unit, tests/integration, tests/contract)
**Target Platform**: Local CLI/TUI (macOS/Linux/Windows)
**Project Type**: CLI + TUI application
**Performance Goals**: Settings updates reflected immediately in UI; note retrieval remains within existing 2s p95 requirement
**Constraints**: Maintain UX consistency; avoid breaking existing profiles; migrations must be backward-safe
**Scale/Scope**: Single-user local profiles; small per-profile DB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Code Quality enforced (format, lint, vet) — ✅ Planned
- Testing standards (cover success + failure paths) — ✅ Planned
- UX consistency (settings behavior aligns with existing UX) — ✅ Planned

Re-check after Phase 1 design: ✅ (no new violations introduced)

## Project Structure

### Documentation (this feature)

```text
specs/008-note-extraction-strategy/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── contracts/           # Phase 1 output (not used; internal-only feature)
```

### Source Code (repository root)

```text
cmd/
internal/
├── app/
├── commands/
├── config/
├── profile/
├── store/
├── tui/
└── vector/

internal/store/migrations/profile/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Single Go module with CLI/TUI in `internal/` and tests in `tests/`.

## Complexity Tracking

No constitution violations required.

## Phase 0: Research

- Confirm current settings persistence for embeddings model (profile.json vs provider_config).
- Identify unused provider_config columns by comparing schema to read/write usage in code.
- Validate TUI settings refresh path after model picker updates.

## Phase 1: Design & Contracts

### Data model updates
- Add embeddings model to provider_config schema (and migrate existing profile settings).
- Remove unused provider_config columns from new profile schemas only (no drop migration).

### UI behavior
- Ensure settings menu values refresh immediately after embedding model selection.
- Keep UI behavior consistent with existing model/extractor flows.

### Contracts
- None (internal CLI/TUI feature).

## Phase 2: Implementation Plan

1. **Schema & migration**
   - Add a migration to introduce an embeddings_model column in provider_config.
   - Migrate existing profile.json embedding model into provider_config for the active profile.
   - Remove unused provider_config columns from new profile schemas only (no drop migration).

2. **Storage layer updates**
   - Extend `store.ProviderConfig` to include embeddings model.
   - Update repository methods to read/write embeddings model.

3. **Settings persistence**
   - Update settings save path to store embeddings model in provider_config (not profile settings).
   - Update settings refresh to read embeddings model from provider_config.

4. **Settings list refresh**
   - Ensure the Settings list entry updates immediately after model selection.
   - Add/update UI commands to re-sync settings list after picker selection.

5. **Tests**
   - Add unit/integration tests for provider_config migration + embeddings model persistence.
   - Add UI test coverage for settings list refresh after changing embeddings model.

6. **QA**
   - Verify settings list updates instantly for model/extractor/embeddings changes.
   - Verify provider_config schema cleanup for new profiles does not break existing profiles.
