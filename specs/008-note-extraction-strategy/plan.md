# Implementation Plan: note-extraction-strategy

**Branch**: `[008-note-extraction-strategy]` | **Date**: 2026-04-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-note-extraction-strategy/spec.md`

## Summary

Refine note extraction by scoring candidates, deduplicating via vector similarity, and surfacing stored notes to users via a brief footer notification. Notes are persisted in the profile’s SQLite database with a vector index for fast relevance and deduplication.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra CLI, Bubble Tea v2, Bubbles v2, Lip Gloss v2, modernc.org/sqlite, internal/vector/hnsw  
**Storage**: Profile-local SQLite database (`~/.noto/profiles/<profile>/memory.db`) and vector index (`~/.noto/profiles/<profile>/memory.vec`)  
**Testing**: `go test ./...`, `make test`  
**Target Platform**: Local CLI/TUI on macOS/Linux  
**Project Type**: CLI/TUI application  
**Performance Goals**: Note extraction + deduplication + retrieval complete within 2 seconds for 95% of chats  
**Constraints**: Offline-capable, profile-local storage, low-latency interactive UX  
**Scale/Scope**: Up to ~10k notes per profile; single-user per profile

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: Use `make fmt`, `make lint`, and `make vet`; reject changes that fail linting or static analysis.
- **Testing Standards Gate**: Add unit/integration coverage for extraction scoring, deduplication logic, vector lookup, and footer notification flow, including error paths.
- **UX Consistency Gate**: Footer notification must follow existing TUI status/notification patterns (terminology, placement, timing). Any deviation documented.
- **Performance Gate**: Measure extraction + retrieval latency in tests or instrumentation; verify 95% of interactions complete within 2 seconds.

## Project Structure

### Documentation (this feature)

```text
specs/008-note-extraction-strategy/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
internal/
├── memory/
├── vector/
├── ui/
└── profiles/

tests/
```

**Structure Decision**: Single Go CLI/TUI project with command entrypoints in `cmd/` and feature logic in `internal/`.

## Complexity Tracking

No constitution violations.
