# Data Model: Messenger Chat UI History Scrolling

## Entity: ConversationHistoryWindow

Represents the currently loaded conversation messages for the active profile/session.

### Fields
- `profile_id` (string, required): Active profile identifier.
- `conversation_id` (string, required): Most recent active conversation for the profile.
- `messages` (ordered list of ChatMessageView, required): Currently loaded slice, oldest -> newest.
- `loaded_count` (int, required): Number of currently loaded conversation messages.
- `load_cap_start` (int, fixed): Initial load size = 10.
- `lazy_batch_size` (int, fixed): Up-scroll lazy-load size = 10.
- `has_older` (bool): Whether older messages exist beyond current window.

### Validation Rules
- `messages` MUST remain chronologically ordered.
- `loaded_count` MUST equal `len(messages)`.
- Initial load MUST return up to 10 messages.
- Lazy-load prepend MUST add at most 10 older messages.

### State Transitions
- `empty` -> `loaded_initial` on successful startup load.
- `loaded_initial`/`loaded_more` -> `loaded_more` on top-boundary lazy-load when `has_older=true`.
- any -> `reloaded` on profile switch.
- any -> `error_nonfatal` when fetch fails, while maintaining interactive UI.

---

## Entity: InputHistoryWindow

Represents in-memory loaded input-history entries used by input-zone scrolling.

### Fields
- `profile_id` (string, required): Active profile identifier.
- `entries` (ordered list of string, required): Loaded input history, oldest -> newest.
- `loaded_count` (int, required): Number of loaded entries.
- `first_load_size` (int, fixed): 3.
- `lazy_batch_size` (int, fixed): 3.
- `loaded_cap` (int, fixed): 12.
- `has_older` (bool): Whether older stored entries remain beyond loaded window.
- `draft_value` (string, optional): Current unsent input to restore when navigating to newest slot.

### Validation Rules
- No preload at startup: `loaded_count` MUST be 0 initially.
- First input scroll-triggered load MUST load exactly 3 latest entries when available.
- Additional backward loads MUST occur in increments of 3.
- `loaded_count` MUST NOT exceed 12.

### State Transitions
- `empty` -> `loaded_initial` on first input-zone scroll request.
- `loaded_initial`/`loaded_more` -> `loaded_more` on further backward scroll when `has_older=true` and `loaded_count<12`.
- any -> `cleared_after_send` immediately after successful send.

---

## Entity: ScrollZone

Represents active mouse-hover area for wheel event routing.

### Values
- `messages`
- `input`
- `outside` (no scrolling action)

### Rules
- Wheel events over `messages` MUST affect conversation viewport only.
- Wheel events over `input` MUST affect input-history window only.
- No overflow handoff between zones.

---

## Entity: ChatMessageView

UI projection of stored message data.

### Fields
- `id` (string)
- `role` (enum: `user`, `assistant`, `command`, `pending`)
- `content` (string)
- `timestamp` (datetime)
- `alignment` (derived enum: `left`, `right`, `inline`)

### Mapping Rules
- `user` -> `right`
- `assistant` -> `left`
- `command` -> `inline`
- `pending` -> transient `inline`/assistant placeholder
