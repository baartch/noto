# Feature Specification: Memory Context Indexing

**Feature Branch**: `005-memory-context`
**Created**: 2026-04-10
**Status**: Draft
**Input**: User description: "Notes are stored in SQLite. Notes are extracted using the current logic. A vector index should be maintained to find notes easier. Notes should be added to the context by relevance. The context should be persistant between session restarts. The context should be maintained automatically and compacted it required."

## Clarifications

### Session 2026-04-10

- Q: How should we limit injected notes? → A: Use a fixed token budget for relevance selection.
- Q: What should the token budget be? → A: 1,500 tokens by default, adjustable via settings.
- Q: How do users open settings? → A: Press `Ctrl+J` to open the settings dialog.
- Q: What should the fallback ranking be when the vector index is unavailable? → A: Importance then recency.
- Q: How should the vector index be generated/updated? → A: Incremental updates on note changes with periodic rebuild/compaction.

### Session 2026-04-27

- Q: Where should the extractor prompt be stored? → A: Profile-local at `<profile>/prompts/extractor.md`.
- Q: How should extraction actions be represented? → A: Per extracted note with `action: add|update`.
- Q: How many notes should extraction return? → A: No fixed count; extract all meaningful notes above quality threshold.
- Q: Where should the system prompt be stored? → A: Profile-local Markdown file in `<profile>/prompts`, not in SQLite.

### Session 2026-05-18

- Q: What dimensions must define context-cache identity? → A: Cache identity includes profile, prompt, notes hash, token budget, and embedding model.
- Q: How should slightly stale cache entries be handled? → A: Serve immediately, then refresh in the background.
- Q: Which events should invalidate or stale cache entries? → A: Note create/update/delete, system prompt changes, token budget changes, and embedding model changes.
- Q: What cache layers are required? → A: Two-level cache with process-local fast cache plus persistent cross-session cache.
- Q: What diagnostics are required? → A: Track hit/miss rate, average rebuild time, and top miss reasons.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Relevant Memory Context (Priority: P1)

As a user, I want Noto to add only the most relevant notes to the context so that responses remain accurate without overflowing the prompt.

**Why this priority**: Relevance-based context keeps conversations focused and performant as memory grows.

**Independent Test**: Create a large set of notes, run a chat turn, and verify that only the top relevant notes appear in the assembled context.

**Acceptance Scenarios**:

1. **Given** a profile has many notes, **When** a chat turn starts, **Then** only the most relevant notes within the configured token budget (default 1,500) are injected into the context.
2. **Given** notes are created or updated, **When** the change is saved, **Then** the vector index is incrementally updated.
3. **Given** the extractor returns multiple notes, **When** extraction completes, **Then** each note includes its own `action` value (`add` or `update`).
4. **Given** periodic maintenance runs, **When** compaction is required, **Then** the index is rebuilt without manual commands.
5. **Given** the note index is available, **When** relevance is computed, **Then** the system uses the vector index to rank notes.
6. **Given** the vector index is missing or stale, **When** a chat turn starts, **Then** the system falls back to importance-then-recency ordering.
7. **Given** the user opens settings, **When** they change the token budget, **Then** subsequent chat turns use the new budget.
8. **Given** no extractor model is configured, **When** notes are extracted, **Then** the system uses the main model and shows a footer warning.
9. **Given** a message contains multiple distinct memory-worthy facts, **When** extraction runs, **Then** all meaningful notes are returned (not an arbitrary fixed count).

---

### User Story 2 - Fast and Correct Cache Reuse (Priority: P2)

As a user, I want context retrieval to be fast while still reflecting configuration and note changes, so responses stay snappy and accurate.

**Why this priority**: Cache quality directly impacts perceived latency and trust in retrieved memory context.

**Independent Test**: Run repeated chat turns across profile reopen/restart and verify immediate reuse for valid entries, stale-while-revalidate behavior for slightly stale entries, and refresh when invalidation events occur.

**Acceptance Scenarios**:

1. **Given** a cache entry exists for the same profile, prompt, notes hash, token budget, and embedding model, **When** context is requested, **Then** the system returns the cached result.
2. **Given** a cache entry is slightly stale but otherwise valid, **When** context is requested, **Then** the system serves the stale entry immediately and starts a background refresh.
3. **Given** a cache entry is refreshed in the background, **When** the next request arrives, **Then** the newer entry is returned.
4. **Given** a profile is reopened after restart, **When** a matching entry exists, **Then** context retrieval uses the persistent cache without a full rebuild.
5. **Given** repeated requests occur within one runtime session, **When** matching data is requested, **Then** the fastest cache tier is used before persistent lookup.

---

### User Story 3 - Observable Cache Health (Priority: P3)

As a maintainer, I want diagnostics for cache behavior so I can quickly identify why retrieval slows down or rebuilds too often.

**Why this priority**: Visibility into hit quality and miss causes enables effective tuning and avoids blind troubleshooting.

**Independent Test**: Trigger a mix of hits/misses and invalidation events, then verify diagnostics expose hit/miss rates, average rebuild time, and ranked miss reasons.

**Acceptance Scenarios**:

1. **Given** context requests are processed, **When** diagnostics are viewed, **Then** hit and miss rates are shown for the observed window.
2. **Given** cache rebuilds occur, **When** diagnostics are viewed, **Then** average rebuild time is reported.
3. **Given** misses happen for different causes, **When** diagnostics are viewed, **Then** top miss reasons are listed (for example, notes changed or prompt changed).

---

## Edge Cases

- If the vector index cannot be loaded, retrieval must still return a deterministic fallback selection.
- If compaction fails, the system logs a warning and continues with existing data.
- If note volume exceeds configured limits, the system truncates by relevance and recency.
- If extractor output is not valid JSON or misses required per-note fields, the system rejects extraction output and logs a warning.
- If `<profile>/prompts/system.md` or `<profile>/prompts/extractor.md` is missing, the system auto-bootstraps defaults and emits a visible warning (non-fatal).
- If the embedding model is changed, prior cache entries are not reused for retrieval decisions.
- If multiple invalidation events happen during an in-flight background refresh, the refresh result is discarded unless it matches the newest cache identity.
- If diagnostics storage is reset or unavailable, retrieval continues and diagnostics resume from the next successful sample.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST keep memory notes stored in the profile SQLite database.
- **FR-002**: The system MUST use the existing extraction logic for creating notes.
- **FR-003**: The system MUST maintain a vector index for memory note retrieval.
- **FR-004**: The system MUST rank notes by relevance when assembling context.
- **FR-005**: The system MUST enforce a token budget when selecting notes for injection (default 1,500).
- **FR-006**: The system MUST allow the token budget to be adjusted via settings.
- **FR-007**: The system MUST open a settings dialog when the user presses `Ctrl+J`.
- **FR-008**: The system MUST persist context retrieval data across app restarts.
- **FR-009**: The system MUST incrementally update the vector index when notes change.
- **FR-010**: The system MUST periodically compact or rebuild the vector index when required.
- **FR-011**: The system MUST fall back to importance-then-recency selection if the index is unavailable.
- **FR-012**: The system MUST use the main model for extraction when no extractor model is configured and display a footer warning.
- **FR-013**: Prompts MUST be persisted as profile-local Markdown files under `<profile>/prompts/` and MUST NOT be stored in SQLite:
  - `system.md` is the canonical system prompt file.
  - `extractor.md` is the canonical extractor prompt file and starts from the current prompt content.
- **FR-014**: The extractor output MUST assign `action: add|update` on each individual extracted note, rather than at the whole-response level.
- **FR-015**: The extractor prompt MUST instruct the LLM to extract as many notes as are meaningful for the input, with no fixed note-count cap.
- **FR-018**: The extractor LLM response MUST be valid JSON containing a notes array.
- **FR-019**: Each note in extractor output MUST include its own `action` field with value `add` or `update`.
- **FR-020**: Notes with `action: update` MUST include `target_id` referencing the existing note id to update.
- **FR-021**: Each extracted note MUST include exactly one category from `fact|progress|blocker|action_item|other`.
- **FR-022**: The system and extractor prompts MUST prioritize accurate, user-useful note extraction over verbosity.
- **FR-023**: The extractor JSON response MUST include top-level `has_new_info` as a boolean (`true|false`).
- **FR-024**: The extractor JSON response MUST include top-level `confidence` as a float in the range `0.0..1.0`.
- **FR-025**: If `<profile>/prompts/system.md` or `<profile>/prompts/extractor.md` is missing, the system MUST auto-create the missing file with defaults and emit a visible warning without failing the flow.
- **FR-026**: Cache identity MUST include all of the following inputs: profile, system prompt content (or equivalent prompt identity), notes hash, token budget, and embedding model.
- **FR-027**: The system MUST NOT return a cache hit when any cache identity input differs from the original entry inputs.
- **FR-028**: If a cache entry is slightly stale and still structurally valid, the system MUST return it immediately and trigger asynchronous revalidation.
- **FR-029**: Asynchronous revalidation MUST update cache contents for subsequent requests without blocking the current request.
- **FR-030**: The system MUST invalidate or mark cache entries stale when notes are created, updated, or deleted.
- **FR-031**: The system MUST invalidate or mark cache entries stale when the system prompt changes.
- **FR-032**: The system MUST invalidate or mark cache entries stale when the token budget changes.
- **FR-033**: The system MUST invalidate or mark cache entries stale when the embedding model changes.
- **FR-034**: The system MUST support two cache layers: a fast in-session cache and a persistent cross-session cache.
- **FR-035**: The system MUST check the in-session cache before persistent cache for retrieval requests.
- **FR-036**: The system MUST record cache diagnostics including hit rate, miss rate, and average rebuild time.
- **FR-037**: The system MUST record and expose the most frequent cache miss reasons.

### Non-Functional Requirements _(mandatory)_

- **NFR-001 Performance**: Context assembly MUST remain responsive with large note volumes.
- **NFR-002 Reliability**: Index maintenance MUST not block chat flows; failures degrade gracefully.
- **NFR-003 UX Consistency**: Context behavior MUST be consistent across sessions.
- **NFR-004 Observability**: Index maintenance should emit warnings on failure.

### Key Entities _(include if feature involves data)_

- **Memory Note**: A stored fact/progress/action item used for context retrieval.
- **Vector Index**: Persistent index used to rank notes by relevance.
- **Context Cache**: Stored assembled prompt context with freshness state and identity inputs.
- **Cache Identity**: The set of request-affecting inputs used to determine whether a cached context is safe to reuse.
- **Cache Diagnostics Snapshot**: Aggregated cache health metrics including hit/miss rates, rebuild timing, and miss-reason ranking.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: 95% of chat turns include only the top ranked relevant notes within the configured token budget (default 1,500).
- **SC-002**: Context assembly remains under 200ms with 10k notes.
- **SC-003**: App restarts preserve index usage without reprocessing >90% of notes.
- **SC-004**: Index maintenance completes without user intervention in 99% of runs.
- **SC-006**: Incremental updates are applied on note changes without blocking chat turns.
- **SC-005**: Fallback selection yields deterministic importance-then-recency results when the index is unavailable.
- **SC-007**: When no extractor model is configured, extraction uses the main model and the footer shows a warning indicator.
- **SC-008**: 100% of accepted extraction payloads conform to the JSON schema (`notes[]`, per-note `action`, `target_id` for updates, valid category).
- **SC-009**: 100% of accepted extraction payloads include valid `has_new_info` (boolean) and `confidence` (0.0..1.0).
- **SC-010**: 100% of missing prompt-file cases (`system.md`/`extractor.md`) auto-bootstrap defaults and emit a visible warning while continuing operation.
- **SC-011**: At least 90% of eligible repeated requests within a session are served from cache without full rebuild.
- **SC-012**: For slightly stale entries, users receive an immediate response and refreshed data is available by the next request in at least 95% of sampled cases.
- **SC-013**: 100% of note changes, prompt changes, token budget changes, and embedding model changes trigger cache stale/invalidation behavior before next retrieval.
- **SC-014**: Diagnostics surface top miss reasons with counts, covering at least 95% of miss events in the reporting window.

## Assumptions

- Note extraction continues to use the current LLM-based extractor.
- Vector index storage is local to the profile directory.
- Relevance ranking uses embeddings from the configured provider.
- “Slightly stale” uses an existing freshness window policy and does not require user intervention.
- Cache diagnostics are accessible to maintainers through existing observability surfaces for the feature area.
