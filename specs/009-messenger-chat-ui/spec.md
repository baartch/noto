# Feature Specification: Messenger-Style Chat UI with History Scrolling

**Feature Branch**: `009-messenger-chat-ui`  
**Created**: 2026-04-26  
**Status**: Draft  
**Input**: User description: "The conversation in noto from the perspective of the user consists of send and received messages. These should be shown similar to a messenger app. scrolling up with the mouse wheel or page up button should go backwards in the history. scrolling down the opposite. When noto opens, it should load the last ten conversation messages (if available). The user should continue right there where they left. scrolling back in history should lazy load small batches of more messages. We already support scrolling in the app. But currently scrolling the mouse wheel scrolls first the user input textarea, and then when it reaches the end, it scrolls the messages history. This is not what I want. When the mouse cursor is in the messages area, scrolling should scroll the history only. When the cursor is in the input textarea, scrolling should scroll the input history only."

## Clarifications

### Session 2026-04-26

- Q: What should the input-history loading policy be? → A: No preload; first scroll loads 3; lazy-load by 3 up to 12; clear loaded input-history after send.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Conversation Continuity on Startup (Priority: P1)

When the user opens noto, they immediately see the most recent messages from their last conversation — not a blank screen. The last 10 messages (or fewer if the history is shorter) are displayed in the messages area, newest at the bottom, so the user can read context and continue right where they left off.

**Why this priority**: This is the foundation of the feature. Without it, every session feels like starting from scratch — the user has no context on open, and all other scrolling improvements are irrelevant if there is nothing to scroll through.

**Independent Test**: Can be tested by sending a few messages, quitting noto, reopening it, and verifying the messages area shows the last conversation messages immediately on startup.

**Acceptance Scenarios**:

1. **Given** the user has a previous conversation with at least 10 messages, **When** they open noto, **Then** the 10 most recent messages are displayed in the messages area at launch.
2. **Given** the user has a previous conversation with fewer than 10 messages, **When** they open noto, **Then** all available messages are shown.
3. **Given** the user has no conversation history at all, **When** they open noto, **Then** the messages area is empty and no error is shown.
4. **Given** messages are loaded on startup, **When** the messages area is rendered, **Then** it is scrolled to the bottom so the most recent message is visible.

---

### User Story 2 - Zone-Aware Mouse Wheel Scrolling (Priority: P2)

When the user moves the mouse cursor into the messages area and scrolls the mouse wheel, only the conversation history scrolls — not the input text area. When the cursor is in the input area, only the input command history cycles — the messages area does not move. There is no "overflow" behavior where scrolling one zone bleeds into the other.

**Why this priority**: The current behavior is disorienting: the wheel first scrolls the textarea, and only after it bottoms out does it affect the messages. Separating the two scroll zones is the single biggest UX improvement in this feature and a prerequisite for comfortable history browsing.

**Independent Test**: Can be tested by positioning the cursor in the messages area and scrolling up — messages should scroll without changing the input field, and vice versa.

**Acceptance Scenarios**:

1. **Given** the cursor is positioned inside the messages area, **When** the user scrolls the mouse wheel up, **Then** the conversation history scrolls backward (older messages) and the input area is unaffected.
2. **Given** the cursor is positioned inside the messages area, **When** the user scrolls the mouse wheel down, **Then** the conversation history scrolls forward (newer messages) and the input area is unaffected.
3. **Given** the cursor is positioned inside the input textarea, **When** the user scrolls the mouse wheel up, **Then** the input command history cycles to an older entry and the messages area is unaffected.
4. **Given** the cursor is positioned inside the input textarea, **When** the user scrolls the mouse wheel down, **Then** the input command history cycles to a newer entry (or restores the draft) and the messages area is unaffected.
5. **Given** no input-history entries are loaded yet in memory, **When** the user first scrolls while cursor is inside the input textarea, **Then** exactly the latest 3 input-history entries are loaded.
6. **Given** the user keeps scrolling upward in the input textarea after reaching the oldest loaded entry, **When** more stored input history exists, **Then** 3 more entries are lazy-loaded per step until 12 total are loaded.
7. **Given** 12 input-history entries are already loaded, **When** the user scrolls further upward, **Then** no additional input-history entries are loaded.

---

### User Story 3 - Lazy Loading Older Messages by Scrolling (Priority: P3)

When the user has scrolled to the top of the currently loaded messages and continues scrolling up (or presses Page Up), the next batch of older messages is loaded from history and prepended to the view — allowing the user to browse their full conversation history without loading everything at once.

**Why this priority**: Builds on P1 and P2. Without lazy loading, very long conversation histories would either be truncated or cause slow startup. Lazy loading keeps startup fast while still making all history reachable.

**Independent Test**: Can be tested by generating a conversation longer than 10 messages, restarting noto, and scrolling to the top of the loaded messages — a further scroll should reveal older messages.

**Acceptance Scenarios**:

1. **Given** there are more messages in history than the currently displayed batch, **When** the user scrolls up to the top of the loaded messages, **Then** the next batch of older messages is loaded and prepended above the existing ones.
2. **Given** all available messages are already loaded, **When** the user scrolls up at the top, **Then** no new messages are fetched and scroll position stays at the top.
3. **Given** a batch is being loaded, **When** the user continues scrolling, **Then** the scroll position is preserved so the previously visible messages remain on screen after the new batch is prepended.
4. **Given** the user is mid-history and sends a new message, **When** the reply arrives, **Then** the view scrolls back to the bottom to show the latest exchange.

---

### Edge Cases

- What happens when a conversation is very long (thousands of messages)? Lazy loading must prevent excessive memory use; batch size should be small and fixed.
- How does the system handle the case where the previous conversation's profile differs from the current one? Only messages belonging to the active profile's conversation should be shown.
- What if message loading fails on startup (e.g., database unavailable)? The app should open normally with an empty messages area and surface a non-fatal error indicator.
- What if the user switches profiles mid-session? The messages area should reset and load the last 10 messages for the newly active profile.
- What happens when the terminal is resized while browsing history? The viewport should re-render without losing scroll position context.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: On startup, the system MUST load and display the last 10 messages (user and assistant turns) from the most recent conversation for the active profile, ordered oldest-to-newest with the newest at the bottom.
- **FR-002**: The messages area MUST be scrolled to the bottom on initial render so the most recent message is immediately visible.
- **FR-003**: When the mouse cursor is positioned inside the messages viewport, mouse wheel scroll events MUST scroll the conversation history only and MUST NOT affect the input area's state.
- **FR-004**: When the mouse cursor is positioned inside the input textarea, mouse wheel scroll events MUST cycle the input command history only and MUST NOT scroll the conversation history.
- **FR-004a**: The system MUST NOT preload input-history entries into memory at startup.
- **FR-004b**: On the first input-history scroll action in the input textarea, the system MUST load exactly the latest 3 stored input-history entries.
- **FR-004c**: When the user scrolls beyond the oldest loaded input-history entry, the system MUST lazy-load older input-history entries in batches of 3.
- **FR-004d**: The system MUST cap loaded input-history entries to 12 total during a single compose session.
- **FR-005**: Scrolling up (mouse wheel or Page Up) in the messages area MUST navigate toward older messages; scrolling down (mouse wheel or Page Down) MUST navigate toward newer messages.
- **FR-006**: When the user reaches the top of the currently loaded messages and triggers an upward scroll, the system MUST fetch the next older batch of messages and prepend them to the view.
- **FR-007**: The lazy-load batch size MUST be small and fixed (default: 10 messages per batch) to bound memory use and render time.
- **FR-008**: After prepending a new batch of older messages, the viewport MUST maintain the user's visual position so previously visible messages remain on screen (no jump).
- **FR-009**: When the user sends a new message or receives a reply, the view MUST scroll to the bottom automatically.
- **FR-009a**: After a message is sent, the system MUST clear in-memory loaded input-history entries.
- **FR-010**: When the active profile changes (e.g., via profile switch), the messages area MUST clear and reload the last 10 messages for the new profile.
- **FR-011**: If message history is unavailable or empty on startup, the messages area MUST render empty without an error blocking use.
- **FR-012**: The user message bubbles MUST be visually right-aligned (as in a typical messenger app) and assistant message bubbles MUST be visually left-aligned.

### Key Entities *(include if feature involves data)*

- **ConversationHistory**: A time-ordered sequence of `chatMessage` entries belonging to a specific profile, sourced from the persistent message store. Has a cursor position indicating how much has been loaded.
- **ScrollZone**: One of two named regions of the TUI — `messages` (the conversation viewport) and `input` (the text entry area). Mouse events are dispatched to the zone under the cursor.
- **LazyLoadCursor**: A pointer (e.g., offset or oldest-loaded message timestamp) that tracks how far back in the history has been fetched, used to request the next batch.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On opening noto after at least one prior conversation, the user sees conversation messages immediately — no additional interaction is required.
- **SC-002**: A user can scroll the mouse wheel while hovering over the messages area without any unintended change to the input field text or command history.
- **SC-003**: A user can scroll the mouse wheel while hovering over the input area to cycle command history without the messages area moving.
- **SC-003a**: Input-history loading behavior matches policy: first scroll loads 3 entries, subsequent backward lazy-loads are in batches of 3, and no more than 12 entries are loaded per compose session.
- **SC-004**: A user with a conversation of 50+ messages can scroll back through the full history from startup using only mouse wheel or Page Up inputs.
- **SC-005**: The application opens in under 2 seconds regardless of how long the conversation history is (lazy loading ensures only 10 messages are fetched on startup).
- **SC-006**: After a profile switch, the messages area reflects the new profile's history within 1 second.

## Assumptions

- The active conversation concept already exists in the data layer; messages are tied to a `conversation_id` which belongs to a profile. The feature reads from the existing store.
- "Last 10 messages" refers to the 10 most recent non-system messages (user and assistant turns) in the most recently active conversation for the profile.
- The existing Bubble Tea viewport component tracks scroll position; the new behavior adds cursor-zone detection on top of it.
- Mobile or touch-only input is out of scope; the feature targets desktop terminal use.
- The visual messenger-style layout (right-aligned user bubbles, left-aligned assistant bubbles) is already implemented in the existing TUI code; this feature wires up the data loading and scroll routing, not a layout redesign.
- Command output messages (role: "command") are treated as system info lines and are included in history display but do not count toward the 10-message startup limit for user/assistant turns.
- Conversation-history lazy-load batch size of 10 is a sensible default and can be a compile-time constant; runtime configurability is out of scope for this feature.
- Input-history lazy loading is intentionally conservative because usage is infrequent: no preload, batch size 3, and maximum 12 loaded entries per compose session.
- "Clear the memory after send" means clearing only the in-memory loaded input-history cache; persisted history in storage remains intact.
