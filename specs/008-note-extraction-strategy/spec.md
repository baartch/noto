# Feature Specification: note-extraction-strategy

**Feature Branch**: `008-note-extraction-strategy`  
**Created**: 2026-04-14  
**Status**: Draft  
**Input**: User description: "Lets refine the note taking implementation. Help me brainstorm the best solution for extracting notes, deciding if its worth to store it and the best way to find the important notes while chatting. Avoid duplicate note taking."

## Clarifications

### Session 2026-04-14

- Q: When should the user see the footer message for newly captured notes? → A: Show the footer message only when a note is actually stored.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Capture valuable notes without duplicates (Priority: P1)

As a user chatting with the assistant, I want the system to capture only meaningful notes and avoid duplicates so my memory store stays concise and trustworthy.

**Why this priority**: This is the core value of note taking; without accurate capture and deduplication, the feature becomes noisy and unusable.

**Independent Test**: Can be fully tested by running a chat session with repeated or low-value statements and verifying only valuable, unique notes are stored.

**Acceptance Scenarios**:

1. **Given** a chat session that includes a clear, actionable fact or preference, **When** the system evaluates the exchange, **Then** it stores exactly one note representing that information.
2. **Given** a chat session that repeats the same fact with minor wording changes, **When** the system evaluates the exchange, **Then** it does not create a duplicate note and links the new context to the existing note.

---

### User Story 2 - Surface important notes during chat (Priority: P2)

As a user continuing a conversation, I want the most relevant notes surfaced at the right time so the assistant can respond with context that matters to me.

**Why this priority**: Retrieval of important notes enables better responses and avoids missing key context.

**Independent Test**: Can be tested by storing multiple notes of varying relevance and verifying only the top relevant notes are surfaced in response to a new prompt.

**Acceptance Scenarios**:

1. **Given** multiple stored notes and a new user prompt, **When** the system searches for relevant notes, **Then** it returns a ranked short list that includes the most relevant notes.

---

### User Story 3 - Review and manage captured notes (Priority: P3)

As a user, I want to see a brief footer notification when a note is actually stored and review what was captured so I can trust and manage my memory store.

**Why this priority**: Transparency and control build user trust and allow corrections when needed.

**Independent Test**: Can be tested by listing stored notes and verifying each entry includes its origin and rationale.

**Acceptance Scenarios**:

1. **Given** a new note is stored, **When** it is captured during a chat, **Then** the system shows a footer message with the note content for roughly 3 seconds.
2. **Given** stored notes, **When** the user requests a review, **Then** the system shows each note with its source context and a brief reason it was stored.

---

### Edge Cases

- What happens when a note candidate has low confidence or ambiguous value?
- How does the system handle partial duplicates where only some attributes overlap?
- What happens when the conversation context changes and previously stored notes become irrelevant?

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST identify note candidates from user-assistant exchanges using clear extraction criteria.
- **FR-002**: System MUST score each note candidate for value (importance, specificity, and future usefulness).
- **FR-003**: System MUST store a note only when its score meets a defined minimum threshold.
- **FR-004**: System MUST detect duplicates by comparing new candidates against existing notes and avoid creating duplicates.
- **FR-005**: System MUST link new supporting context to an existing note when a duplicate is detected.
- **FR-006**: System MUST retrieve and rank notes for a new user prompt based on relevance to the current conversation.
- **FR-007**: System MUST limit surfaced notes to a concise list that fits the conversation context.
- **FR-008**: System MUST show a footer notification when a note is stored, displaying the note content for roughly 3 seconds.
- **FR-009**: System MUST expose a user-facing review that includes each note’s origin and storage rationale.

### Non-Functional Requirements _(mandatory)_

- **NFR-001 Code Quality**: Changes MUST pass formatting, linting, and static analysis rules
  defined by the project.
- **NFR-002 Testing Standards**: Changes MUST include automated tests for new/changed behavior,
  including negative/error paths where applicable.
- **NFR-003 UX Consistency**: User-facing changes MUST follow established UX patterns
  (terminology, interaction flows, visual behavior) or document approved deviations.
- **NFR-004 Performance**: Critical note extraction and retrieval MUST complete within 2 seconds for 95%
  of interactions, with verification steps documented.

### Key Entities _(include if feature involves data)_

- **Note**: A stored memory item with content, metadata (source, timestamp), and a value score.
- **Note Candidate**: A provisional extracted item with evidence, confidence, and duplicate match status.
- **Note Retrieval Result**: A ranked list of notes selected for a specific prompt with relevance scores.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: At least 90% of stored notes are rated “useful” by users in review feedback.
- **SC-002**: Duplicate note creation is reduced to fewer than 2 per 100 stored notes in test sessions.
- **SC-003**: For prompts with relevant memories, the correct note appears in the top 5 results at least 95% of the time.
- **SC-004**: Users can complete a note review and correction action in under 2 minutes.
- **SC-005**: 0 lint/format violations in CI for feature scope.
- **SC-006**: All new/changed behavior covered by automated tests.
- **SC-007**: 95% of chat interactions that require note retrieval return results within 2 seconds.

## Assumptions

- Users expect notes to reflect stable facts, preferences, and long-term context rather than ephemeral chat content.
- The feature scope focuses on text chat interactions and does not include external document ingestion.
- The system can access the existing note storage and retrieval infrastructure.
- Users have access to a standard review interface for viewing stored notes.
