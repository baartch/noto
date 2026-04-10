# Research: Memory Context Indexing

## Decision: Incremental vector indexing + periodic rebuild

**Rationale**: Incremental updates on note changes keep relevance fresh without full rebuild cost; periodic compaction controls index growth.

**Alternatives considered**:
- Full rebuild on every retrieval (too expensive for large profiles)
- Manual batch rebuild only (risks stale retrieval and missed updates)

## Decision: Token budget (default 1,500) for note injection

**Rationale**: Token budgeting keeps prompts bounded while supporting many short notes.

**Alternatives considered**:
- Fixed note count (can overflow prompt with long notes)
- Hybrid (more complex without clear need)

## Decision: Fallback ranking = importance then recency

**Rationale**: Deterministic and uses existing metadata.

**Alternatives considered**:
- Recency-only (ignores importance)
- Recency then importance (less aligned with current note ordering)
