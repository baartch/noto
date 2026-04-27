# Implementation Plan: Bubble Tea TUI Standard

**Branch**: `004-bubbletea-tui` | **Date**: 2026-04-27 | **Spec**: `/specs/004-bubbletea-tui/spec.md`
**Input**: Feature specification from `/specs/004-bubbletea-tui/spec.md`

## Summary

Standardize all TUI flows on Bubble Tea v2 and prefer Bubbles v2 primitives with Lip Gloss v2 styling. Keep input/footer anchored during overlays, enforce keybindings (`Ctrl+D`, `Ctrl+L`, `Ctrl+H`, `Ctrl+J`), and always render footer telemetry. Footer telemetry aggregates usage/cost from provider `usage` payloads across main chat, extractor, and embeddings models.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra CLI, Bubble Tea v2, Bubbles v2, Lip Gloss v2, OpenAI-compatible provider adapter  
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) + profile-local files (`memory.vec`, prompt files)  
**Testing**: `go test` with unit/integration/contract suites (`tests/unit`, `tests/integration`, `tests/contract`)  
**Target Platform**: Cross-platform terminal environments (Linux/macOS primary)  
**Project Type**: CLI application with interactive TUI  
**Performance Goals**: No perceptible lag for normal typing/navigation; footer metrics update within normal render loop cadence  
**Constraints**: Footer always visible; overlay filtering must not collapse list; usage totals must include main + extractor + embeddings usage/cost; `Ctrl+J` must open settings dialog  
**Scale/Scope**: All existing and new TUI interaction flows in `internal/tui` and related feature modules

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Code Quality Is Enforced**: PASS — plan includes refactor boundaries and lint/format/vet requirements.
- **II. Testing Standards Are Non-Negotiable**: PASS — plan requires tests for positive/negative UI behavior and usage aggregation.
- **III. User Experience Consistency First**: PASS — plan preserves keybindings, footer/help behavior, and overlay anchoring conventions.

No constitutional violations requiring complexity justification.

## Project Structure

### Documentation (this feature)

```text
specs/004-bubbletea-tui/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tui-flows.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── noto/

internal/
├── tui/
├── provider/
├── observe/
├── chat/
├── memory/
└── profile/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Use the existing single-project Go CLI layout. Implement UI behavior primarily in `internal/tui`, with usage/cost sourcing from provider response paths and validated by integration/contract tests.

## Phase 0: Research Outcomes

- Usage source standardized to API `usage` payloads from responses/chunks.
- Aggregation scope includes main model + extractor + embeddings model usage/cost.
- Footer contract fixed to always-visible telemetry + profile/model/version/help.
- Missing `usage` handling defined as no-op (retain last totals, no estimation).

(See `/specs/004-bubbletea-tui/research.md`.)

## Phase 1: Design Outcomes

- Data entities expanded to include session usage accumulator and per-response usage snapshots.
- Contract updated for footer telemetry completeness and cross-model accounting.
- Quickstart updated with verification flow for usage parsing and aggregation behavior.
- Agent context updated to reference this plan in `AGENTS.md`.

## Post-Design Constitution Re-Check

- **I. Code Quality Is Enforced**: PASS — concrete modules and responsibilities identified.
- **II. Testing Standards Are Non-Negotiable**: PASS — explicit validation added for usage parsing/aggregation and footer rendering.
- **III. User Experience Consistency First**: PASS — no UX deviations introduced; requirements made more testable.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
