# UI Contract: Scroll Behavior and History Loading

## Scope
Defines observable interaction contract for chat viewport and input textarea scroll behaviors.

## Inputs
- Mouse wheel up/down with cursor in messages zone
- Mouse wheel up/down with cursor in input zone
- Page Up / Page Down in chat view
- Send action (Enter)
- Profile switch

## Contract Rules

1. **Wheel zone isolation**
   - If cursor is in messages zone, wheel input MUST mutate only conversation viewport state.
   - If cursor is in input zone, wheel input MUST mutate only input-history navigation state.

2. **Page key routing**
   - Page Up MUST always scroll conversation history toward older messages, regardless of hover zone.
   - Page Down MUST always scroll conversation history toward newer messages, regardless of hover zone.

3. **Conversation startup**
   - On app open, conversation view MUST show up to last 10 messages and position at bottom.

4. **Conversation lazy-load**
   - On upward boundary scroll in messages zone, system MUST prepend next batch of up to 10 older messages.
   - After prepend, viewport continuity MUST be preserved (no disruptive jump).

5. **Input-history lazy-load**
   - No input-history preload on startup.
   - On first input-zone wheel history scroll, load latest 3 entries.
   - Additional backward wheel requests load older entries in batches of 3.
   - Total in-memory input-history entries MUST NOT exceed 12.

6. **Send reset behavior**
   - After send, in-memory input-history window MUST be cleared.
   - Persisted history remains unchanged.

7. **Profile switch behavior**
   - Conversation window MUST reset and load new profile's latest 10 messages.
   - Input-history window MUST reset to empty.

8. **Failure handling**
   - History read failures MUST be non-fatal and surfaced via inline error feedback.

9. **Baseline alignment guard (manual validation gate)**
   - User bubbles remain right-aligned and assistant bubbles remain left-aligned (FR-012 baseline; no behavior change in this feature).
