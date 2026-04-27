# Implementation Plan: Memory Context Indexing

**Branch**: `005-memory-context` | **Date**: 2026-04-27 | **Spec**: `/specs/005-memory-context/spec.md`
**Input**: Feature specification from `/specs/005-memory-context/spec.md`

## Summary

Add profile-local Markdown prompt storage (`<profile>/prompts/system.md`, `<profile>/prompts/extractor.md`) and tighten extraction contracts so the extractor always returns valid JSON with top-level `has_new_info` and `confidence`, plus as many meaningful notes as warranted, each note carrying its own `action` (`add|update`), mandatory `target_id` on updates, and constrained category (`fact|progress|blocker|action_item|other`). Missing prompt files are auto-bootstrapped with defaults and a visible warning. Keep existing relevance ranking, token budgeting, persistence, and vector index maintenance behavior.

## Technical Context

**Language/Version**: Go 1.26+  
**Primary Dependencies**: Cobra CLI, Bubble Tea v2, Bubbles v2, Lip Gloss v2, modernc.org/sqlite, OpenAI-compatible provider adapter, internal pure-Go HNSW index  
**Storage**: Profile-local SQLite (`~/.noto/profiles/<profile>/memory.db`) for notes/cache + profile-local files (`~/.noto/profiles/<profile>/prompts/*.md`, `memory.vec`)  
**Testing**: `go test ./...` + contract/integration tests for extractor payload validation and prompt persistence  
**Target Platform**: Cross-platform terminal environments (Linux/macOS primary)  
**Project Type**: CLI + TUI application  
**Performance Goals**: Context assembly <200ms at 10k notes; prompt file load/save non-blocking for normal interactions  
**Constraints**: Deterministic fallback ranking, robust malformed-JSON handling, no DB storage for system/extractor prompts, missing prompt files are non-fatal (auto-bootstrap defaults + visible warning)  
**Scale/Scope**: Single-user local profiles, up to 10k+ notes per profile, multiple profile directories

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Code Quality Is Enforced**: PASS — plan includes focused modules for prompt storage and extraction validation, explicit error handling for invalid payloads.
- **II. Testing Standards Are Non-Negotiable**: PASS — includes unit + integration/contract coverage for JSON schema rules, update target validation, prompt file persistence.
- **III. User Experience Consistency First**: PASS — keeps existing settings and fallback UX, adds clear behavior on malformed extractor output (reject + warning).

Post-Design Re-check (after Phase 1 artifacts):
- **I. Code Quality Is Enforced**: PASS
- **II. Testing Standards Are Non-Negotiable**: PASS
- **III. User Experience Consistency First**: PASS

## Project Structure

### Documentation (this feature)

```text
specs/005-memory-context/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── context-retrieval.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/
internal/
├── config/            # profile + prompt file persistence
├── memory/            # note extraction + validation orchestration
├── vector/            # hnsw indexing and retrieval
└── tui/               # settings and runtime UX feedback
tests/
├── integration/
└── contract/
```

**Structure Decision**: Keep single-project Go CLI/TUI architecture and implement prompt persistence in profile config/memory layers; add contract tests for extractor JSON response semantics.

## Phase 0: Research Plan

1. Confirm best-practice storage layout and migration for prompt values moving from DB/config to profile-local Markdown files.
2. Define strict JSON contract for extractor responses with per-note action semantics and update targeting.
3. Define validation/error strategy for malformed or partially valid extraction output.
4. Define prompt-writing guidance for accuracy and user utility.

## Phase 1: Design Plan

1. Extend data model with Prompt File and Extraction Payload entities/validation, including top-level `has_new_info` and `confidence`.
2. Update contract docs for retrieval + extraction schema invariants, including top-level metadata requirements.
3. Update quickstart to include prompt file and schema validation scenarios (with top-level metadata checks).
4. Update agent context reference in `AGENTS.md` to point to this plan.

## Phase 2: Task Planning Approach

Generate implementation tasks that sequence: file persistence layer → extractor schema validation (including top-level `has_new_info`/`confidence`) → extraction pipeline integration → settings wiring → tests (unit/integration/contract) → docs.

## Complexity Tracking

No constitution violations requiring justification.
