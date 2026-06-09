# Feature Specification: Memory Context Indexing

**Feature Branch**: `005-memory-context`
**Created**: 2026-04-10
**Status**: Draft
**Input**: User description: "Notes are stored in SQLite. Notes are extracted using the current logic. A vector index should be maintained to find notes easier. Notes should be added to the context by relevance. The context should be persistant between session restarts. The context should be maintained automatically and compacted it required." Updated with: "The provided context is not comprehensive enough. Currently the context is like docs/CONTEXT_AT_A_GLANCE.txt. Instead of having previous session summary and long-term memory notes, the context should include all recent notes plus older weekly and monthly rollups, with weekly and monthly summaries generated whenever a new week or month begins. Conversation summaries are no longer needed and should be removed. The LLM should also receive search tools: keyword-based vector search and time-range note search."

## Clarifications

### Session 2026-04-10

- Q: How do users open settings? → A: Press `Ctrl+J` to open the settings dialog.
- Q: What should the fallback ranking be when the vector index is unavailable? → A: Importance then recency.
- Q: How should the vector index be generated/updated? → A: Incremental updates on note changes with periodic rebuild/compaction.

### Session 2026-04-27

- Q: Where should the extractor prompt be stored? → A: Profile-local at `<profile>/prompts/extractor.md`.
- Q: How should extraction actions be represented? → A: Per extracted note with `action: add|update`.
- Q: How many notes should extraction return? → A: No fixed count; extract all meaningful notes above quality threshold.
- Q: Where should the system prompt be stored? → A: Profile-local Markdown file in `<profile>/prompts`, not in SQLite.

### Session 2026-05-18

- Q: What dimensions must define context-cache identity? → A: Cache identity includes profile, prompt, assembled memory state, embedding model, and timeline window settings.
- Q: How should slightly stale cache entries be handled? → A: Serve immediately, then refresh in the background.
- Q: Which events should invalidate or stale cache entries? → A: Note create/update/delete, system prompt changes, embedding model changes, timeline window setting changes, and summary-state changes.
- Q: What cache layers are required? → A: Two-level cache with process-local fast cache plus persistent cross-session cache.
- Q: What diagnostics are required? → A: Track hit/miss rate, average rebuild time, and top miss reasons.

### Session 2026-06-09

- Q: What should replace previous-session summaries and long-term-memory note snippets in the assembled context? → A: A time-layered context consisting of recent raw notes, mid-range weekly summaries, and older monthly summaries according to the configured day/week/month windows.
- Q: When should weekly and monthly summaries be created? → A: Automatically when the system enters a new week or a new month.
- Q: Are conversation summaries still needed? → A: No, conversation summaries should be removed from the feature scope and from assembled context.
- Q: What retrieval tools should be available to the LLM? → A: A keyword-based vector search tool and a time-range note search tool.
- Q: Which timeline windows should be user-adjustable in settings? → A: Number of days with all notes (default 30; any integer greater than 0 is allowed), number of weeks with weekly summaries (default 8), and number of months with monthly summaries (default all remaining months).
- Q: How are timeline boundaries computed to avoid gaps? → A: The raw-notes layer always covers at least the configured rolling-day window and then fills backward to the preceding Monday so there is no gap before weekly summaries; the weekly-summary layer starts with the first full week before that boundary and extends as needed to cover at least the first day of the following monthly-summary month so there is no gap before monthly summaries; the monthly-summary layer switches only on completed calendar periods; if the monthly-summary window is bounded instead of `all remaining`, anything older than that cutoff is excluded from default context.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Time-Layered Context Assembly (Priority: P1)

As a user, I want the assistant to receive a broader, time-aware memory context so it can remember recent detail while still preserving older history in a compact form.

**Why this priority**: This directly improves response quality by replacing overly narrow context with a predictable historical view.

**Independent Test**: Populate a profile with notes spanning several months, start a new chat turn, and verify that the assembled context includes the configured rolling-day raw-note window, the configured number of following weeks as weekly summaries, and the configured monthly-summary window for older months.

**Acceptance Scenarios**:

1. **Given** the setting for raw-note days is 30, **When** context is assembled, **Then** all notes from at least the last 30 rolling days are included and the window extends backward to the preceding Monday so there is no gap before weekly summaries.
2. **Given** the setting for weekly-summary weeks is 8, **When** context is assembled, **Then** at least the configured number of weeks before the raw-note boundary are represented through weekly summaries and the weekly layer extends as needed to cover at least the first day of the following monthly-summary month so there is no gap.
3. **Given** the setting for monthly-summary months is all remaining months, **When** context is assembled, **Then** all older history is represented through monthly summaries.
4. **Given** a user changes any of the timeline-window settings, **When** the next context is assembled, **Then** the assembled context reflects the updated day-, week-, and month-based window sizes.
5. **Given** an existing profile currently uses previous-session summaries or long-term-memory note snippets, **When** the new context model is active, **Then** those older context sections are no longer included.
6. **Given** no historical notes exist for one or more periods, **When** context is assembled, **Then** the system omits empty sections without failing the request.

---

### User Story 2 - Automatic Summary Rollups (Priority: P2)

As a user, I want older notes to be rolled up automatically into weekly and monthly summaries so the assistant can preserve long-term continuity without overwhelming the prompt.

**Why this priority**: Without automatic rollups, the richer context model becomes expensive to maintain and degrades as history grows.

**Independent Test**: Move a profile across a week boundary and a month boundary with eligible notes present, then verify that new weekly and monthly summaries are created automatically and become available to later context assembly.

**Acceptance Scenarios**:

1. **Given** a new week begins and the prior week has notes that are not yet summarized weekly, **When** the profile is next processed, **Then** a weekly summary is created automatically for that completed week.
2. **Given** a new month begins and the prior month has content that should be rolled up, **When** the profile is next processed, **Then** a monthly summary is created automatically for that completed month.
3. **Given** a period has already been summarized, **When** summary generation runs again, **Then** the system does not create duplicate summaries for the same period.
4. **Given** a weekly summary exists for a time range that later becomes part of an older monthly rollup, **When** monthly summaries are used for older history, **Then** the monthly summary becomes the retained representation for that older month.
5. **Given** summary generation fails for a period, **When** context is assembled, **Then** the system continues using the best available underlying memory for that period and records the failure for follow-up.

---

### User Story 3 - LLM Memory Search Tools (Priority: P3)

As the assistant, I want explicit memory search tools so I can look up additional information by topic or date range instead of relying only on preloaded context.

**Why this priority**: Search tools reduce prompt bloat and let the assistant retrieve precise supporting memory on demand.

**Independent Test**: Ask questions that require topic lookup and date-bounded lookup, then verify the assistant can access keyword-based search results and time-range search results from the memory store.

**Acceptance Scenarios**:

1. **Given** the assistant needs notes related to a topic, **When** it invokes keyword-based memory search, **Then** it receives relevant matching notes ranked for that topic.
2. **Given** the assistant needs notes from a specific date range, **When** it invokes time-range note search, **Then** it receives notes constrained to that requested period.
3. **Given** both preloaded context and search results are available, **When** the assistant answers the user, **Then** it can use the search results as supporting memory without changing the default assembled context structure.
4. **Given** a search request returns no matches, **When** the assistant uses the tool, **Then** the tool returns an empty result set with no failure of the overall interaction.

---

### User Story 4 - Fast and Correct Cache Reuse (Priority: P4)

As a user, I want context retrieval to remain fast while still reflecting timeline rollups, configuration changes, and note changes.

**Why this priority**: Cache quality still determines perceived responsiveness and trust in memory retrieval.

**Independent Test**: Run repeated chat turns across profile reopen/restart and verify immediate reuse for valid entries, stale-while-revalidate behavior for slightly stale entries, and refresh when notes, summaries, or retrieval settings change.

**Acceptance Scenarios**:

1. **Given** a cache entry exists for the same profile, prompt, selected memory state, timeline window settings, and embedding model, **When** context is requested, **Then** the system returns the cached result.
2. **Given** a cache entry is slightly stale but otherwise valid, **When** context is requested, **Then** the system serves the stale entry immediately and starts a background refresh.
3. **Given** summary rollups or note changes alter the assembled timeline context, **When** the next retrieval occurs, **Then** the prior cache entry is not reused as a valid hit.
4. **Given** a profile is reopened after restart, **When** a matching entry exists, **Then** context retrieval uses the persistent cache without a full rebuild.
5. **Given** repeated requests occur within one runtime session, **When** matching data is requested, **Then** the fastest cache tier is used before persistent lookup.

---

### User Story 5 - Observable Memory Health (Priority: P5)

As a maintainer, I want diagnostics for context assembly, rollups, and retrieval behavior so I can quickly identify why memory quality degrades or rebuilds happen too often.

**Why this priority**: Visibility is necessary to tune the new timeline-based model and detect summary-generation gaps.

**Independent Test**: Trigger a mix of hits, misses, rollup creation events, and search requests, then verify diagnostics expose hit/miss rates, average rebuild time, rollup status, and ranked miss reasons.

**Acceptance Scenarios**:

1. **Given** context requests are processed, **When** diagnostics are viewed, **Then** hit and miss rates are shown for the observed window.
2. **Given** cache rebuilds occur, **When** diagnostics are viewed, **Then** average rebuild time is reported.
3. **Given** misses happen for different causes, **When** diagnostics are viewed, **Then** top miss reasons are listed.
4. **Given** weekly or monthly summaries are created or skipped, **When** diagnostics are viewed, **Then** recent rollup activity and failures are visible.

## Edge Cases

- If no notes exist for the configured raw-note day window, the recent-notes section is omitted without creating placeholder noise.
- If the configured number of monthly-summary months is limited instead of set to all remaining months, older history outside that configured window is excluded from the assembled default context.
- If a week or month boundary is crossed while the app is not running, summaries are generated the next time the relevant profile is processed.
- If a period contains too little content for a meaningful summary, the system still preserves that period without inventing unsupported details.
- If weekly summaries do not yet exist for a portion of the configured weekly-summary window, the system uses the best available source for that period until rollup generation completes.
- If monthly summaries do not yet exist for older history, the system uses the best available source for that period until rollup generation completes.
- If a note is edited after a weekly or monthly summary has been created, the affected summary becomes eligible for regeneration before it is trusted as current.
- If the vector index cannot support keyword search temporarily, standard context assembly still works and the search tool reports no results rather than failing the conversation.
- If a time-range search spans periods represented by both raw notes and summaries, the returned result set preserves the requested time boundaries and clearly distinguishes the returned record types.
- If summary generation fails, retrieval continues with best available data and the failure is surfaced in diagnostics.
- If conversation summaries still exist from older data, they are ignored for context assembly once the new timeline model is active.
- If the assembled timeline context exceeds the internal prompt-fitting limit, the system reduces context internally by dropping the oldest monthly-summary coverage first before reducing newer timeline layers.
- If additional reduction is still required after monthly-summary coverage has been exhausted, the system continues reducing older timeline coverage before newer timeline coverage.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST keep memory notes stored in the profile SQLite database.
- **FR-002**: The system MUST use the existing extraction logic for creating notes.
- **FR-003**: The system MUST maintain a vector index for memory note retrieval.
- **FR-004**: The system MUST assemble memory context as a time-layered view rather than as a previous-session summary plus vector-selected long-term notes.
- **FR-005**: The system MUST allow users to configure in settings the number of recent rolling days included as raw notes, with a default of 30 days.
- **FR-006**: The raw-note setting MUST accept any integer number of days greater than 0.
- **FR-007**: The system MUST allow users to configure in settings the number of following weeks represented by weekly summaries, with a default of 8 weeks.
- **FR-008**: The system MUST allow users to configure in settings the number of older months represented by monthly summaries, with a default of all remaining months.
- **FR-008a**: The weekly-summary setting MUST accept only integer week values greater than 0.
- **FR-008b**: The monthly-summary setting MUST accept either `all remaining` or an integer number of months greater than 0.
- **FR-009**: The assembled context MUST include all notes from the configured recent raw-note day window for the active profile.
- **FR-010**: The recent raw-note window MUST cover at least the configured rolling-day span and extend backward to the preceding Monday so there is no gap before the weekly-summary layer.
- **FR-011**: The assembled context MUST include weekly summaries for the configured number of weeks immediately preceding the raw-note window, starting with the first full week before the raw-note boundary.
- **FR-012**: The weekly-summary layer MUST extend as needed to cover at least the first day of the following monthly-summary month so there is no gap before the monthly-summary layer.
- **FR-013**: The assembled context MUST include monthly summaries for the configured older-history monthly-summary window, switching only on completed calendar periods.
- **FR-014**: If the monthly-summary setting is a bounded integer instead of `all remaining`, the default assembled context MUST exclude any history older than that configured monthly-summary cutoff.
- **FR-015**: The system MUST persist context retrieval data across app restarts.
- **FR-016**: The system MUST incrementally update the vector index when notes change.
- **FR-017**: The system MUST periodically compact or rebuild the vector index when required.
- **FR-018**: The system MUST fall back to deterministic non-vector ordering if vector-based retrieval is unavailable where needed.
- **FR-019**: The system MUST use the main model for extraction when no extractor model is configured and display a visible warning.
- **FR-020**: Prompts MUST be persisted as profile-local Markdown files under `<profile>/prompts/` and MUST NOT be stored in SQLite:
  - `system.md` is the canonical system prompt file.
  - `extractor.md` is the canonical extractor prompt file and starts from the current prompt content.
- **FR-021**: The extractor output MUST assign `action: add|update` on each individual extracted note, rather than at the whole-response level.
- **FR-022**: The extractor prompt MUST instruct the LLM to extract as many notes as are meaningful for the input, with no fixed note-count cap.
- **FR-023**: The system MUST automatically generate a weekly summary for each completed week as soon as the system detects entry into a new week.
- **FR-024**: The system MUST automatically generate a monthly summary for each completed month as soon as the system detects entry into a new month.
- **FR-025**: The extractor LLM response MUST be valid JSON containing a notes array.
- **FR-026**: Each note in extractor output MUST include its own `action` field with value `add` or `update`.
- **FR-027**: Notes with `action: update` MUST include `target_id` referencing the existing note id to update.
- **FR-028**: Each extracted note MUST include exactly one category from `fact|progress|blocker|action_item|other`.
- **FR-029**: The system and extractor prompts MUST prioritize accurate, user-useful note extraction over verbosity.
- **FR-030**: The extractor JSON response MUST include top-level `has_new_info` as a boolean (`true|false`).
- **FR-031**: The extractor JSON response MUST include top-level `confidence` as a float in the range `0.0..1.0`.
- **FR-032**: If `<profile>/prompts/system.md` or `<profile>/prompts/extractor.md` is missing, the system MUST auto-create the missing file with defaults and emit a visible warning without failing the flow.
- **FR-033**: Cache identity MUST include all of the following inputs: profile, system prompt content (or equivalent prompt identity), assembled memory state, embedding model, and the configured timeline window settings.
- **FR-034**: The system MUST NOT return a cache hit when any cache identity input differs from the original entry inputs.
- **FR-035**: If a cache entry is slightly stale and still structurally valid, the system MUST return it immediately and trigger asynchronous revalidation.
- **FR-036**: Asynchronous revalidation MUST update cache contents for subsequent requests without blocking the current request.
- **FR-037**: The system MUST invalidate or mark cache entries stale when notes are created, updated, or deleted.
- **FR-038**: The system MUST invalidate or mark cache entries stale when the system prompt changes.
- **FR-039**: The system MUST invalidate or mark cache entries stale when the embedding model changes.
- **FR-040**: The system MUST invalidate or mark cache entries stale when any timeline window setting changes.
- **FR-041**: The system MUST support two cache layers: a fast in-session cache and a persistent cross-session cache.
- **FR-042**: The system MUST check the in-session cache before persistent cache for retrieval requests.
- **FR-043**: The system MUST record diagnostics including hit rate, miss rate, and average rebuild time.
- **FR-044**: The system MUST record and expose the most frequent cache miss reasons.
- **FR-045**: The system MUST remove conversation summaries from context assembly and from ongoing feature scope for memory retrieval.
- **FR-046**: The system MUST NOT require a previous-session summary in order to assemble context successfully.
- **FR-047**: The system MUST make a keyword-based memory search tool available to the LLM for retrieving relevant notes on demand.
- **FR-048**: The keyword-based memory search tool MUST return results relevant to the supplied keywords.
- **FR-049**: The system MUST make a time-range memory search tool available to the LLM for retrieving notes by date boundaries.
- **FR-050**: The time-range memory search tool MUST return only records whose timestamps fall within the requested range.
- **FR-051**: The assembled context MUST distinguish between raw recent notes, weekly summaries, and monthly summaries so the assistant can interpret the historical granularity correctly.
- **FR-052**: Weekly and monthly summaries MUST be stored and reused as first-class memory artifacts rather than regenerated for every request.
- **FR-053**: The system MUST prevent duplicate weekly summaries for the same profile and calendar week.
- **FR-054**: The system MUST prevent duplicate monthly summaries for the same profile and calendar month.
- **FR-055**: If a stored summary becomes outdated because underlying notes changed, the system MUST mark that summary for regeneration before it is relied on as current context.
- **FR-056**: Time-range search MUST be able to return both raw notes and summary records when both fall within the requested period.
- **FR-057**: The system MUST preserve context assembly when one or more summary periods are missing by using the best available memory for those periods until summaries are generated.
- **FR-058**: If assembled timeline context must be reduced to fit the internal prompt budget, the system MUST drop the oldest monthly-summary coverage first before reducing newer timeline layers.
- **FR-059**: If additional reduction is still required after monthly-summary coverage has been exhausted, the system MUST continue reducing older timeline coverage before newer timeline coverage.

### Non-Functional Requirements _(mandatory)_

- **NFR-001 Performance**: Context assembly MUST remain responsive as profile history grows across months and years.
- **NFR-002 Reliability**: Summary generation and index maintenance MUST not block chat flows; failures degrade gracefully.
- **NFR-003 UX Consistency**: The same profile history MUST produce the same context composition rules across sessions.
- **NFR-004 Observability**: Summary generation, context assembly, and cache behavior should emit actionable diagnostics on failure.
- **NFR-005 Comprehensiveness**: The default assembled context should preserve more useful historical coverage than the previous session-summary-plus-snippets model.

### Key Entities _(include if feature involves data)_

- **Memory Note**: A stored fact, progress item, blocker, action item, or other durable detail captured from conversations.
- **Weekly Summary**: A rollup representing one completed calendar week of notes for a profile.
- **Monthly Summary**: A rollup representing one completed calendar month of notes for a profile.
- **Timeline Context Window**: The assembled history view composed of recent raw notes, mid-range weekly summaries, and older monthly summaries.
- **Vector Search Request**: A keyword-driven lookup for memory records relevant to a topic.
- **Time-Range Search Request**: A lookup for memory records constrained to a requested start and end time.
- **Context Cache**: Stored assembled prompt context with freshness state and identity inputs.
- **Cache Identity**: The set of request-affecting inputs used to determine whether a cached context is safe to reuse.
- **Diagnostics Snapshot**: Aggregated health metrics including hit/miss rates, rebuild timing, rollup activity, and miss-reason ranking.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: 100% of assembled contexts follow the configured structure for raw-note days, weekly-summary weeks, and monthly-summary months.
- **SC-002**: At least 95% of eligible week transitions produce the missing weekly summary before or during the next successful profile processing cycle.
- **SC-003**: At least 95% of eligible month transitions produce the missing monthly summary before or during the next successful profile processing cycle.
- **SC-004**: 100% of context assemblies succeed without requiring a previous-session summary.
- **SC-005**: Conversation summaries contribute to 0% of newly assembled contexts after rollout of this feature revision.
- **SC-006**: At least 90% of keyword-based memory searches return at least one relevant result when matching notes exist.
- **SC-007**: 100% of successful time-range searches return only records within the requested date range.
- **SC-008**: At least 90% of eligible repeated requests within a session are served from cache without full rebuild.
- **SC-009**: For slightly stale entries, users receive an immediate response and refreshed data is available by the next request in at least 95% of sampled cases.
- **SC-010**: 100% of note changes, summary changes, prompt changes, embedding model changes, and timeline window setting changes trigger stale or invalidation behavior before the next retrieval.
- **SC-011**: Diagnostics surface top miss reasons with counts, covering at least 95% of miss events in the reporting window.
- **SC-012**: At least 90% of evaluated responses to questions about older history are judged by maintainers as having sufficient historical context without needing manual restatement from the user.

## Assumptions

- Note extraction continues to use the current extraction approach for creating raw notes.
- By default, the recent raw-note window is 30 days, the weekly-summary window is 8 weeks, and the monthly-summary window covers all remaining older months.
- The raw-note setting can be adjusted to any integer number of days greater than 0, the weekly-summary setting can be adjusted only to integer week values greater than 0, and the monthly-summary setting can be adjusted to `all remaining` or an integer number of months greater than 0.
- The recent raw-note window uses at least the configured rolling-day span and extends backward to the preceding Monday so there is no gap before the weekly-summary layer.
- The weekly-summary layer starts with the first full week before the raw-note boundary.
- The monthly-summary layer begins only after the weekly-summary layer and switches only on completed calendar periods.
- If the monthly-summary setting is bounded instead of `all remaining`, anything older than that monthly cutoff is excluded from the default assembled context.
- Weekly and monthly summaries are profile-local artifacts that can be stored, reused, and regenerated when stale.
- Search tools are intended for assistant use during conversations rather than as an end-user command surface in this phase.
- If multiple memory sources are available for the same time range, the system prefers the summary level appropriate to that period in the default assembled context.
- Existing cache, prompt, and index capabilities remain in scope and must adapt to the new timeline-based context model.
- Token fitting remains an internal implementation concern rather than a user-facing setting; when reduction is necessary, the oldest monthly-summary coverage is dropped first, and any further reduction continues from older timeline coverage toward newer coverage.
