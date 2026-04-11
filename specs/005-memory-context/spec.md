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

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Relevant Memory Context (Priority: P1)

As a user, I want Noto to add only the most relevant notes to the context so that responses remain accurate without overflowing the prompt.

**Why this priority**: Relevance-based context keeps conversations focused and performant as memory grows.

**Independent Test**: Create a large set of notes, run a chat turn, and verify that only the top relevant notes appear in the assembled context.

**Acceptance Scenarios**:

1. **Given** a profile has many notes, **When** a chat turn starts, **Then** only the most relevant notes within the configured token budget (default 1,500) are injected into the context.
2. **Given** notes are created or updated, **When** the change is saved, **Then** the vector index is incrementally updated.
3. **Given** periodic maintenance runs, **When** compaction is required, **Then** the index is rebuilt without manual commands.
4. **Given** the note index is available, **When** relevance is computed, **Then** the system uses the vector index to rank notes.
5. **Given** the vector index is missing or stale, **When** a chat turn starts, **Then** the system falls back to importance-then-recency ordering.
6. **Given** the user opens settings, **When** they change the token budget, **Then** subsequent chat turns use the new budget.
7. **Given** no extractor model is configured, **When** notes are extracted, **Then** the system uses the main model and shows a footer warning.

---

### User Story 2 - Persistent Context Across Sessions (Priority: P2)

As a user, I want context retrieval to persist across restarts so that memory behavior is consistent between sessions.

**Why this priority**: Persisted context avoids reprocessing and keeps memory continuity intact.

**Independent Test**: Restart the app and verify that context retrieval reuses stored notes and index state without reinitialization.

**Acceptance Scenarios**:

1. **Given** a profile with notes and an index, **When** the app restarts, **Then** retrieval uses the existing index without re-importing notes.
2. **Given** context cache entries exist, **When** the profile is reopened, **Then** the cache is reused until invalidated.

---

### User Story 3 - Automatic Context Maintenance (Priority: P2)

As a maintainer, I want context maintenance (index updates, compaction) to run automatically so that memory relevance stays accurate without manual intervention.

**Why this priority**: Automatic maintenance reduces operational overhead and keeps memory reliable.

**Independent Test**: Add notes, trigger retrieval, and confirm the index is updated or compacted when required.

**Acceptance Scenarios**:

1. **Given** new notes are created, **When** they are stored, **Then** the vector index is updated automatically.
2. **Given** the index is out of date or too large, **When** retrieval runs, **Then** the system compacts/rebuilds the index without manual commands.

---

## Edge Cases

- If the vector index cannot be loaded, retrieval must still return a deterministic fallback selection.
- If compaction fails, the system logs a warning and continues with existing data.
- If note volume exceeds configured limits, the system truncates by relevance and recency.

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

### Non-Functional Requirements _(mandatory)_

- **NFR-001 Performance**: Context assembly MUST remain responsive with large note volumes.
- **NFR-002 Reliability**: Index maintenance MUST not block chat flows; failures degrade gracefully.
- **NFR-003 UX Consistency**: Context behavior MUST be consistent across sessions.
- **NFR-004 Observability**: Index maintenance should emit warnings on failure.

### Key Entities _(include if feature involves data)_

- **Memory Note**: A stored fact/progress/action item used for context retrieval.
- **Vector Index**: Persistent index used to rank notes by relevance.
- **Context Cache**: Stored assembled prompt context with expiration.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: 95% of chat turns include only the top ranked relevant notes within the configured token budget (default 1,500).
- **SC-002**: Context assembly remains under 200ms with 10k notes.
- **SC-003**: App restarts preserve index usage without reprocessing >90% of notes.
- **SC-004**: Index maintenance completes without user intervention in 99% of runs.
- **SC-006**: Incremental updates are applied on note changes without blocking chat turns.
- **SC-005**: Fallback selection yields deterministic importance-then-recency results when the index is unavailable.
- **SC-007**: When no extractor model is configured, extraction uses the main model and the footer shows a warning indicator.

## Assumptions

- Note extraction continues to use the current LLM-based extractor.
- Vector index storage is local to the profile directory.
- Relevance ranking uses embeddings from the configured provider.
