# Quickstart: Cross-Conversation History Scrolling

## Goal
Validate profile-wide backward history scrolling across conversations with boundary separators and existing input-zone behavior.

## Steps

1. Run app (`make run`) with a profile containing multiple conversations.
2. Confirm startup loads latest 10 messages and starts at bottom.
3. Scroll up in messages area until crossing conversation boundary.
4. Verify older conversation messages appear and a thin separator with conversation start date is displayed.
5. Continue scrolling to ensure loading proceeds through all profile history.
6. At absolute history top, further upward scroll should no-op (no crash, no duplicate loads).
7. Verify Page Up/Page Down always scroll messages history regardless of hover zone.
8. Verify wheel in input area continues to use input-history policy (3/3/12) and does not move messages viewport.
9. Send a message and confirm viewport snaps to bottom; input in-memory history window resets.
10. Simulate history load failure and verify non-fatal inline error while interaction remains possible.
11. Manually confirm FR-012 baseline bubble alignment unchanged.

## Validation Commands
- `make fmt`
- `make lint`
- `make vet`
- `make test`
