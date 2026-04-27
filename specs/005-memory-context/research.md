# Research: Memory Context Indexing (Prompt Files + Extraction Contract)

## Decision: Store prompts as profile-local Markdown files in `<profile>/prompts/`

**Rationale**: File-based prompt storage makes prompts transparent, versionable, and easy to inspect/edit per profile while removing schema coupling from SQLite.

**Alternatives considered**:
- Store prompts in SQLite rows (rejected: less transparent and harder to override manually)
- Global app-level prompt files (rejected: not profile-scoped)

## Decision: Enforce strict extractor JSON payload contract

**Rationale**: A hard contract prevents ambiguous parser behavior and ensures deterministic handling of add/update semantics.

**Alternatives considered**:
- Free-form text extraction (rejected: brittle parsing)
- Partially structured JSON with optional required fields (rejected: leads to silent data quality issues)

## Decision: Per-note action semantics with required `target_id` on updates

**Rationale**: Mixed add/update operations in one extraction response require note-local intent to avoid global-action ambiguity.

**Alternatives considered**:
- Single top-level action (rejected: cannot safely represent mixed note outcomes)
- Auto-resolve update target by fuzzy matching only (rejected: too error-prone without explicit id)

## Decision: Constrain note categories to `fact|progress|blocker|action_item|other`

**Rationale**: Controlled vocabulary improves downstream ranking/filtering and avoids taxonomy drift.

**Alternatives considered**:
- Open-ended category strings (rejected: inconsistent taxonomy)
- Smaller category set (rejected: loses useful distinctions)

## Decision: Accuracy-first prompt policy for user utility

**Rationale**: The extraction/system prompts should bias toward reliable, high-signal notes to improve retrieval quality and user trust.

**Alternatives considered**:
- Maximize note count aggressively (rejected: introduces noise)
- Generic summarization prompt (rejected: weaker extraction precision)

## Decision: Reject invalid extraction payloads and log warnings

**Rationale**: Failing closed on invalid JSON/required fields protects data integrity and keeps behavior observable.

**Alternatives considered**:
- Best-effort partial ingestion (rejected: hidden inconsistencies)
- Silent drop of malformed fields (rejected: hard to debug)
