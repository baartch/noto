# Data Model: Cross-Conversation History Scrolling

## Entity: ProfileHistoryWindow

Represents the currently loaded visible message slice for the active profile across multiple conversations.

### Fields
- `profile_id` (string, required)
- `items` (ordered list of `HistoryItem`, required): oldest -> newest in viewport model
- `loaded_count` (int, required)
- `has_older` (bool, required)
- `initial_load_size` (int, fixed = 10)
- `lazy_batch_size` (int, fixed = 10)

### Validation Rules
- `items` MUST remain chronologically ordered by message timestamp.
- `loaded_count` MUST equal number of loaded message items (excluding separators).
- Lazy-load prepend MUST add at most 10 message items per fetch.

### State Transitions
- `empty` -> `loaded_initial`
- `loaded_initial`/`loaded_more` -> `loaded_more` on top-boundary load
- any -> `error_nonfatal` on retrieval failure

---

## Entity: HistoryItem

Union-like item used by renderer.

### Variants
1. `MessageItem`
   - `message_id` (string)
   - `conversation_id` (string)
   - `role` (`user|assistant|command|pending`)
   - `content` (string)
   - `timestamp` (datetime)
2. `ConversationBoundaryItem`
   - `conversation_id` (string)
   - `conversation_started_at` (datetime)
   - `label` (string, formatted date)

### Rules
- Boundary item MUST appear before the first message of an older conversation when crossing conversation ID.
- Boundary item MUST NOT duplicate for contiguous messages in same conversation.

---

## Entity: InputHistoryWindow

Unchanged policy for input-zone history browsing.

### Fields
- `entries` (ordered list)
- `loaded_count` (int)
- `first_load_size` = 3
- `lazy_batch_size` = 3
- `loaded_cap` = 12

### Rules
- No preload on startup.
- First input-zone wheel load = latest 3.
- Additional backward load = +3 up to cap 12.
- Clear in-memory window after send.
