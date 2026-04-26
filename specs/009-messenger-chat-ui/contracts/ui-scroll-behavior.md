# UI Contract: Cross-Conversation Scroll Behavior

## Scope
Defines required UI behavior for profile-wide history scrolling with conversation separators.

## Contract Rules

1. **Profile-wide backward history**
   - Backward scroll in messages area MUST continue into older conversations in the same active profile.

2. **Conversation boundary marker**
   - When crossing into older conversation messages, render a single-character-height separator line formatted like `-- YYYY-MM-DD HH:MM MST ---------------------`.
   - Date/time formatting MUST use Go local time (`time.Local`) with fixed layout `2006-01-02 15:04 MST`.
   - The right-side dashes MUST expand to fill remaining viewport width.

3. **Wheel routing**
   - Wheel in messages zone affects messages history only.
   - Wheel in input zone affects input-history only.

4. **Page key routing**
   - Page Up/Page Down MUST always scroll messages history regardless of hover zone.

5. **Lazy loading behavior**
   - Initial load: latest 10 messages from profile-wide history.
   - Older loads: prepend batches of up to 10 messages.
   - Preserve visual anchor after prepend.

6. **Failure behavior**
   - Paging/read failures are non-fatal and surfaced inline.

7. **Input history policy**
   - No preload; first load 3; lazy +3; cap 12; clear after send.

8. **Manual baseline guard**
   - User/assistant bubble alignment remains unchanged (FR-012 baseline).
