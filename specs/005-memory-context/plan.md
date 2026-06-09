# Implementation Plan: Memory Context Timeline & Tooling

**Branch**: `005-memory-context` | **Date**: 2026-06-09 | **Spec**: [specs/005-memory-context/spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-memory-context/spec.md`

## Summary

Enhance the existing memory implementation without redesigning the product shape: replace previous-session-summary-driven context with a configurable time-layered context window, add automatic weekly/monthly rollups, expose LLM-accessible memory search through OpenRouter tool calling, preserve fast cache reuse under the broader context model, and surface model context-window capacity and current usage percentage in the chat footer next to token telemetry.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra CLI, Bubble Tea v2 + Bubbles v2 + Lip Gloss v2, modernc.org/sqlite, existing provider adapter, internal vector packages, OpenRouter-compatible tool-calling payloads  
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for notes, summaries, cache, provider config, and message history; profile-local vector index file (`memory.vec`); profile-local prompt files under `prompts/`  
**Testing**: `go test ./...`, plus `make test`, `make lint`, `make fmt`  
**Target Platform**: Cross-platform CLI/TUI runtime (Linux/macOS/Windows terminals)  
**Project Type**: Single-project CLI/TUI application  
**Performance Goals**: Keep context assembly responsive during chat turns, preserve fast repeated retrieval through L1/L2 cache reuse, keep tool-calling round-trips bounded so memory search remains conversationally useful, and show footer telemetry without introducing visual jitter  
**Constraints**: Must stay close to the current design principles and existing architecture; must remove reliance on conversation summaries for default context; must support user-adjustable timeline windows; must use OpenRouter tool-calling semantics for search tools; must derive model max context from provider model metadata (`context_length`) when available; must degrade gracefully when metadata or tool support is unavailable  
**Scale/Scope**: Expand the current memory feature from cache hardening only to end-to-end timeline context assembly, rollup generation, LLM search tools, provider model metadata usage, and footer telemetry updates for active chat sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Code Quality Is Enforced**: PASS — the plan extends existing memory/provider/TUI modules instead of introducing a parallel subsystem, preserves focused module boundaries, and requires the standard validation commands `make fmt`, `make lint`, and `make test`.
- **Testing Standards Are Non-Negotiable**: PASS — the plan requires unit and integration coverage for timeline window selection, rollup generation and regeneration, tool-call request/response handling, context-capacity metadata handling, footer telemetry formatting, and cache invalidation under settings/model changes.
- **User Experience Consistency First**: PASS — the design keeps the existing chat/TUI interaction model, adds footer telemetry in the existing status area, and uses tool calling behind the scenes rather than introducing new end-user flows.

Post-Design Re-check: PASS — Phase 1 artifacts keep the current UX model, preserve existing architecture patterns, and add measurable validation guidance without violating constitution gates.

## Project Structure

### Documentation (this feature)

```text
specs/005-memory-context/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── context-retrieval.md
│   ├── memory-search-tools.md
│   └── footer-telemetry.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
└── noto/

internal/
├── app/
│   └── chat_cmd.go
├── cache/
├── chat/
│   ├── pipeline.go
│   └── session.go
├── config/
├── memory/
│   ├── retrieval.go
│   ├── processor.go
│   ├── extractor.go
│   └── summary_rollups.go
├── provider/
│   ├── adapter.go
│   ├── models.go
│   └── openai_compatible.go
├── store/
├── tui/
│   ├── footer_view.go
│   └── model.go
└── vector/

tests/
├── integration/
│   ├── memory/
│   ├── provider/
│   └── tui/
└── unit/
    ├── memory/
    ├── provider/
    └── tui/
```

**Structure Decision**: Use the existing single-project Go CLI/TUI structure and extend current memory, provider, chat session, and footer telemetry paths rather than creating new top-level services.

## Phase 0: Research Plan

1. Confirm timeline-window semantics for raw-note days, weekly-summary weeks, and monthly-summary months, including rejection of invalid zero weekly/monthly values and bounded monthly-history behavior.
2. Determine rollup generation and regeneration rules for week/month boundary transitions, missed-runtime catch-up, and stale-summary replacement.
3. Determine how to represent and expose keyword/vector search and time-range DB search through OpenRouter tool-calling request/response loops while staying compatible with current provider abstractions.
4. Confirm how to fetch and store model `context_length` metadata from the OpenRouter models API and how to use it as footer telemetry for the active model.
5. Define cache identity and invalidation changes needed for timeline-window settings, summary state changes, and tool-capable context assembly.
6. Define footer telemetry behavior when model context metadata is missing, stale, or unavailable.

## Phase 1: Design Plan

1. Update `research.md` with resolved decisions for timeline windows, rollups, OpenRouter tool calling, model metadata, and footer telemetry.
2. Update `data-model.md` with entities for weekly summaries, monthly summaries, timeline settings, tool-call descriptors/results, and provider model metadata.
3. Replace the narrow cache contract with a broader `context-retrieval.md` covering timeline assembly, configurable windows, summary fallback behavior, and cache identity.
4. Add `memory-search-tools.md` documenting tool schemas, tool-call lifecycle, and result-shape contracts for keyword and time-range search.
5. Add `footer-telemetry.md` documenting how token usage, max context size, and usage percentage appear in the footer and what happens when max context is unknown.
6. Rewrite `quickstart.md` with validation scenarios for context composition, rollup generation, OpenRouter tool calls, cache invalidation, and footer telemetry.
7. Update `AGENTS.md` so the active plan reference points to `specs/005-memory-context/plan.md`.

## Phase 2: Implementation Planning (for `/speckit.tasks`)

- Add profile settings for configurable raw-note day windows (any integer greater than 0) plus weekly-summary week and monthly-summary month windows, with weekly-summary values restricted to integers greater than 0 and monthly-summary values restricted to integers greater than 0 or `all_remaining`, while raw-note windows always fill backward to the preceding Monday so there is no gap before weekly summaries and weekly-summary coverage extends as needed to reach at least the first day of the following monthly-summary month.
- Replace session-summary-based context assembly with a timeline-window selector that uses raw notes, weekly summaries, and monthly summaries.
- Implement weekly/monthly rollup creation, deduplication, and regeneration triggers when notes or periods change.
- Extend storage models and migrations for summary artifacts and summary freshness/versioning.
- Expose keyword/vector and time-range memory search as OpenRouter-compatible tools in the provider completion loop, with orchestration kept in `internal/chat/` and provider transport/schema support kept in `internal/provider/`.
- Extend provider model metadata loading to capture `context_length` and tool-support capability from OpenRouter model objects, cache/fetch it for the active model during startup/profile switch/model change flows, and propagate it into session stats.
- Update footer telemetry to show tokens plus max-context usage percentage and context maximum on the left side next to existing token information.
- Extend cache identity and invalidation to include timeline window settings and summary-state changes while keeping token fitting as an internal implementation concern rather than a user-facing setting.
- Define internal token fitting so that when assembled context must be reduced to fit the prompt, the oldest monthly-summary coverage is dropped first before reducing newer timeline layers, and any additional reduction continues from older timeline coverage toward newer coverage.
- Add automated tests covering timeline composition, rollup catch-up, duplicate summary prevention, tool calls and invalid tool inputs, provider metadata parsing, missing/unknown context-length handling, mixed raw+summary time-range results, footer rendering, and internal token-fitting reduction order.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
