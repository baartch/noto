# Feature Specification: Noto Profile Memory CLI

**Feature Branch**: `001-build-profile-memory-cli`  
**Created**: 2026-03-27  
**Status**: Draft  
**Input**: User description: "Build the following: Noto = local CLI chatbot for multi-purpose consulting..."

## Clarifications

### Session 2026-03-27

- Q: What is the local data protection scope? → A: Encrypt provider credentials only; keep profile DB/prompt plaintext local files.
- Q: What is the reliability strategy for local DB corruption? → A: Auto-repair, else restore last local backup.
- Q: What is the backup frequency policy? → A: Periodic + on session end.
- Q: What is the local observability policy? → A: Structured logs + local metrics.
- Q: What is explicitly out of scope for this release? → A: Multi-user sync, cloud backup, and vector memory as source of truth.
- Q: What is the slash command scope inside chat? → A: Full parity with CLI commands.
- Q: How should ambiguous slash commands be handled? → A: Require explicit selection when multiple matches exist.
- Q: When should slash command suggestions be visible? → A: Only when input starts with `/`.
- Q: What slash syntax style should be canonical? → A: Hierarchical style (e.g., `/profile list`, `/prompt show`).
- Q: How should unknown slash commands be handled? → A: Show explicit error with top matching suggestions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start and Chat with a Profile (Priority: P1)

As a user, I can start the terminal app, select or create a consulting profile, and chat in that
profile so I can get role-specific assistance immediately.

**Why this priority**: Core product value is profile-based consulting chat in terminal.

**Independent Test**: Launch app on a machine with zero, one, and multiple profiles; verify
startup behavior and successful chat in active profile.

**Acceptance Scenarios**:

1. **Given** no profiles exist, **When** user starts the app, **Then** the app requires creating
   a first profile before chat starts.
2. **Given** exactly one profile exists, **When** user starts the app, **Then** that profile is
   auto-selected and chat opens.
3. **Given** multiple profiles exist, **When** user starts the app, **Then** user is prompted to
   select a profile or default profile before chat opens.
4. **Given** an active profile, **When** user sends a message, **Then** the assistant responds
   using the active profile context.

---

### User Story 2 - Persistent Memory Continuity (Priority: P2)

As a user, I want the app to remember key facts, progress, blockers, and action items from prior
conversations in the same profile, and reuse an efficient local context cache between sessions,
so future chats are continuous, fast, and cost-efficient.

**Why this priority**: Persistent memory is the main differentiator.

**Independent Test**: Complete one conversation with key facts and action items; restart app and
continue chat in same profile; verify prior context is used, startup context is loaded from local
cache when available, and behavior remains correct after cache refresh.

**Acceptance Scenarios**:

1. **Given** an active profile conversation, **When** a session ends, **Then** key notes are
   captured and stored for that profile and a reusable local session-handoff summary is saved.
2. **Given** prior memory exists in a profile, **When** user starts a new conversation in that
   profile, **Then** relevant prior notes are used to inform responses.
3. **Given** prior cached context exists for that profile, **When** user starts a new
   conversation, **Then** the app reuses cached context and avoids reconstructing full context
   from raw history unless needed.
4. **Given** cached context is stale, missing, or invalid, **When** user starts chatting,
   **Then** the app regenerates context from persisted profile memory and continues normally.

---

### User Story 3 - Manage Profiles and Prompts Safely (Priority: P3)

As a user, I can create, list, select, rename, and delete profiles, and view/edit each profile’s
system prompt, so each consulting role stays isolated and configurable.

**Why this priority**: Profile lifecycle and editable behavior control are required for safe,
long-term multi-purpose use.

**Independent Test**: Perform full profile lifecycle plus prompt view/edit; verify isolation and
deletion confirmation behavior.

**Acceptance Scenarios**:

1. **Given** existing profiles, **When** user runs profile management commands, **Then** changes
   are reflected correctly in subsequent startup and chat sessions.
2. **Given** a profile prompt is edited, **When** user starts chat in that profile, **Then**
   responses follow the updated prompt behavior.
3. **Given** user attempts profile deletion, **When** confirmation is not strong/explicit,
   **Then** deletion is blocked.
4. **Given** two distinct profiles, **When** user chats in one profile, **Then** memory and
   prompt behavior from the other profile is never used.

### Edge Cases

- Profile name duplicates are rejected with a clear corrective message.
- Active profile is deleted only after explicit confirmation and a safe fallback selection.
- User starts app while profile data is missing/corrupted; app reports issue and offers recovery
  path without exposing data from other profiles.
- Encrypted provider credentials cannot be read; app prompts secure reconfiguration without
  exposing credential material.
- Provider becomes unavailable mid-chat; app preserves transcript and allows retry or provider
  switch.
- A slash command is valid in CLI but missing in chat; app treats this as parity failure and
  reports a corrective error.
- Slash input matches multiple commands; app requires explicit user selection before execution.
- Unknown slash command is entered; app shows explicit error and top matching command suggestions.
- Memory retrieval returns no relevant results; app continues chat without failure.
- Cached context is stale or corrupted; app invalidates and rebuilds cache from persisted memory.
- Cache references removed memory items; app repairs cache automatically without cross-profile
  leakage.
- Profile database is corrupted; app attempts automatic repair and, on failure, restores the
  latest local backup with a clear user notice.
- Backup is unavailable or stale; app reports the recovery gap and starts protected fallback flow
  without mixing data from other profiles.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST run as an interactive terminal chatbot experience.
- **FR-002**: System MUST allow users to chat through multiple model providers without
  provider lock-in.
- **FR-003**: System MUST keep all profile data and conversation memory on the user’s local
  machine.
- **FR-004**: System MUST support profile creation when no profile exists.
- **FR-005**: System MUST auto-select the only profile when exactly one exists.
- **FR-006**: System MUST prompt for profile/default selection when multiple profiles exist.
- **FR-007**: System MUST provide commands to create, list, select, rename, and delete profiles.
- **FR-008**: System MUST provide commands to show and edit the active profile prompt at any
  time.
- **FR-009**: System MUST auto-capture conversation notes per profile, including facts,
  progress, blockers, and action items.
- **FR-010**: System MUST use stored profile memory in future chats for continuity and
  personalization.
- **FR-011**: System MUST maintain a per-profile local context cache that can be reused across
  chat sessions to reduce context reconstruction work.
- **FR-012**: System MUST treat persisted profile memory as the source of truth and use cache as
  a performance layer only.
- **FR-013**: System MUST invalidate or refresh cached context when profile prompt, profile
  memory, or profile metadata changes.
- **FR-014**: System MUST enforce strict isolation so memory, prompts, and caches never cross
  profiles.
- **FR-015**: System MUST require strong explicit confirmation before profile deletion.
- **FR-016**: System MUST organize local profile data under a default user-local application
  root directory.
- **FR-017**: System MUST support fast retrieval of relevant prior memory during chat.
- **FR-018**: System MUST keep prompt-driven behavior configurable by users, with no hardcoded
  consulting niche behavior in core flows.
- **FR-019**: System MUST encrypt provider credentials at rest, while profile database, prompts,
  and cache remain local files protected by OS-level file permissions.
- **FR-020**: System MUST attempt automatic repair when a profile database is corrupted and,
  if repair fails, restore from the latest local backup for that profile.
- **FR-021**: System MUST create profile-local backups periodically and at session end to support
  bounded data-loss recovery.
- **FR-022**: System MUST expose every CLI command in chat via slash command format with
  equivalent behavior, validation, and side effects.
- **FR-023**: System MUST require explicit user selection when slash input maps to multiple
  command matches; ambiguous slash input MUST NOT auto-execute.
- **FR-024**: System MUST show command suggestions only while the current chat input is in slash
  command mode (input begins with `/`).
- **FR-025**: System MUST use hierarchical slash command syntax as canonical format (e.g.,
  `/profile list`, `/prompt show`) consistent with CLI command structure.
- **FR-026**: System MUST return explicit error feedback and top matching suggestions when an
  unknown slash command is entered.

### Non-Functional Requirements *(mandatory)*

- **NFR-001 Code Quality**: Changes MUST pass formatting, linting, and static analysis rules
  defined by the project.
- **NFR-002 Testing Standards**: Changes MUST include automated tests for new/changed behavior,
  including negative/error paths where applicable.
- **NFR-003 UX Consistency**: User-facing changes MUST follow established UX patterns
  (terminology, interaction flows, visual behavior) or document approved deviations.
- **NFR-004 Performance**: Changes MUST define measurable performance targets and verification
  steps for critical paths.
- **NFR-005 Privacy**: User content and profile memory MUST remain local by default; no external
  persistence of user data is allowed.
- **NFR-006 Data Protection**: Provider credentials MUST be encrypted at rest; non-credential
  profile artifacts MAY remain plaintext local files under user-controlled OS permissions.
- **NFR-007 Reliability**: Corruption recovery MUST be deterministic: auto-repair first, backup
  restore second, and clear user notification of outcome.
- **NFR-008 Recovery Point**: Backup cadence MUST combine periodic snapshots and session-end
  snapshots to limit potential lost work between recoveries.
- **NFR-009 Observability**: System MUST emit structured local logs and profile-scoped local
  metrics for startup, retrieval, cache, provider calls, and recovery events.

### Key Entities *(include if feature involves data)*

- **Profile**: A user-defined consulting context with a unique name, editable behavior prompt,
  and isolated memory scope.
- **Conversation**: A chronological session of user and assistant messages under one active
  profile.
- **Memory Note**: Structured record extracted from conversations, categorized as fact,
  progress, blocker, or action item, tied to one profile.
- **Session Handoff Summary**: Compact per-profile carry-forward context generated at session end
  to bootstrap future sessions without replaying full prior transcripts.
- **Context Cache Entry**: Reusable per-profile local cache artifact containing assembled context
  inputs for chat startup or early turns, linked to cache validity metadata.
- **Provider Configuration**: User-selectable model access settings that enable chatting across
  multiple providers without changing user workflows.
- **Slash Command**: Chat command invocation using canonical hierarchical syntax mapped to the
  same command semantics as CLI mode.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of first-time users can start a first chat by creating a profile in under
  3 minutes.
- **SC-002**: 95% of users with one existing profile enter chat in one step (no manual profile
  selection).
- **SC-003**: 95% of users with multiple profiles can select the intended profile and start chat
  in under 30 seconds.
- **SC-004**: In validation scenarios, at least 90% of prior-session action items and blockers
  are reflected in subsequent same-profile chats.
- **SC-005**: In repeat-session benchmarks, median time to first contextual response improves by
  at least 30% when valid local context cache exists.
- **SC-006**: In isolation tests, 0 cross-profile memory/cache leaks are observed.
- **SC-007**: 100% of profile deletion attempts require explicit confirmation before data removal.
- **SC-008**: 95% of users report that profile behavior changes take effect immediately after
  prompt edits.
- **SC-009**: In benchmark scenarios, memory recall does not noticeably delay conversational
  turn-taking for typical profile histories.
- **SC-010**: Cache rebuild success rate is at least 99% when cache is missing or invalid.
- **SC-011**: In corruption simulations, at least 99% of affected profiles recover via
  auto-repair or backup restore without cross-profile impact.
- **SC-012**: In recovery drills, maximum unrecoverable work is limited to activity since the
  last periodic or session-end backup.
- **SC-013**: In validation runs, 100% of startup, memory retrieval, cache invalidation,
  provider error, and recovery flows produce structured log records and local metric updates.
- **SC-014**: In UI validation runs, command suggestion lists are shown for 100% of slash-mode
  inputs and 0% of non-slash chat inputs.

## Assumptions

- Users are individual operators running the app in a local terminal environment.
- Users can provide valid credentials/configuration for at least one model provider.
- The feature scope focuses on local single-user workflows, not multi-user shared deployments.
- Memory extraction categories (facts, progress, blockers, action items) are sufficient for
  MVP continuity needs.
- Session-handoff summaries and local context cache are bounded in size to keep storage and
  runtime overhead predictable.
- Out of scope for this release: multi-user sync, cloud backup, and vector memory as source of
  truth (vector may be added later as secondary retrieval support).
- Advanced semantic memory enhancements are out of scope for this feature and can be introduced
  later without changing core profile workflows.
