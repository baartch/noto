# Implementation Plan: Memory Context Indexing

**Branch**: `005-memory-context` | **Date**: 2026-04-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-memory-context/spec.md`

## Summary

Implement relevance-based memory retrieval with a persistent vector index, configurable 1,500-token default budget, importance-then-recency fallback, extractor-model fallback to main model with footer warning, and automatic index maintenance. Ensure context caching persists across restarts and maintenance does not block chat.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: charm.land/bubbletea/v2, charm.land/bubbles/v2, charm.land/lipgloss/v2, Cobra, modernc.org/sqlite
**Storage**: Per-profile SQLite DB + profile-local vector index file
**Testing**: `go test ./...`, integration tests in `tests/integration`
**Target Platform**: Terminal (Linux/macOS/Windows)
**Project Type**: CLI/TUI application
**Performance Goals**: Context assembly under 200ms with 10k notes
**Constraints**: Token budget default 1,500 (adjustable), deterministic fallback ordering
**Scale/Scope**: Single-profile memory retrieval with incremental indexing and periodic compaction

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Gate**: Run `golangci-lint run` and `go test ./...` before merge.
- **Testing Standards Gate**: Add/adjust tests for relevance selection, fallback ordering, and persistence across restarts.
- **UX Consistency Gate**: Ensure settings dialog (Ctrl+J) and footer warning align with existing TUI patterns and terminology.
- **Performance Gate**: Validate context assembly latency under 200ms with 10k notes.

## Project Structure

### Documentation (this feature)

```text
specs/005-memory-context/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md
```

### Source Code (repository root)

```text
internal/
├── memory/
│   ├── extractor.go
│   ├── retrieval.go
│   └── ...
├── vector/
│   ├── index.go
│   ├── sync.go
│   ├── rebuild.go
│   └── hnsw/
├── store/
│   ├── memory_note_repo.go
│   ├── vector_manifest_repo.go
│   └── ...
├── tui/
│   └── ...
└── commands/

specs/005-memory-context/

tests/
├── integration/
└── contract/
```

**Structure Decision**: Single Go CLI/TUI project with memory and vector logic under `internal/memory` and `internal/vector`.

## Plan

### Phase 0: Research
- Confirm incremental vector indexing strategy and compaction triggers.
- Validate token-budgeted selection approach for short notes.

### Phase 1: Design
- Define context selection flow (vector ranking → token budget → fallback ordering).
- Define index maintenance cadence and compaction trigger rules.
- Specify settings storage for token budget and Ctrl+J dialog behavior.
- Define extractor fallback behavior and footer warning text.

### Phase 2: Implementation
- Persist token budget setting and wire Ctrl+J settings dialog.
- Add relevance selection using vector index and token budget.
- Implement importance-then-recency fallback when index missing/stale.
- Implement extractor-model fallback to main model with footer warning.
- Implement incremental index updates on note changes and periodic compaction/rebuild.
- Ensure cached context persists across restarts.

### Phase 3: Validation
- Run `go test ./...` and lint checks.
- Validate latency budget with large note sets.
- Run quickstart scenarios for relevance, persistence, and fallback.

## Complexity Tracking

> **No constitution violations identified.**
