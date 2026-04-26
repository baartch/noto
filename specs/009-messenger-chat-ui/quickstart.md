# Quickstart: Messenger Chat UI History Scrolling

## Goal
Validate conversation history continuity, zone-aware scrolling, and bounded lazy loading for both conversation and input history.

## Prerequisites
- Project builds locally
- Active profile exists with prior conversation and input history
- Terminal supports mouse events

## Steps

1. **Build and run**
   - `make build`
   - `make run`

2. **Startup continuity check**
   - Confirm latest 10 conversation messages are displayed on open (or fewer if unavailable).
   - Confirm viewport opens at bottom (latest visible).

3. **Messages-zone wheel routing check**
   - Move cursor over messages viewport.
   - Scroll up/down and verify only conversation viewport changes.
   - Verify input text and input-history selection remain unchanged.

4. **Input-zone wheel routing + lazy load check**
   - Move cursor over input textarea.
   - First upward history scroll should load exactly 3 latest input-history entries.
   - Continue upward scrolling; verify additional loads happen in batches of 3.
   - Verify total loaded input-history entries never exceed 12.
   - Verify messages viewport does not move during input-zone scrolling.

5. **Send reset check**
   - Submit an input.
   - Verify in-memory input-history window resets (next history scroll triggers fresh first-load of 3).

6. **Conversation lazy-load check**
   - In messages zone, scroll to top and continue scrolling up.
   - Verify older messages prepend in batches of 10.
   - Verify visual anchor remains stable after prepend (no jump).

7. **Profile switch check**
   - Switch profile.
   - Verify conversation window resets and loads new profile latest 10.
   - Verify input-history window resets to empty.

8. **Failure handling check**
   - Simulate/force message store read failure.
   - Verify app remains interactive and displays non-fatal inline error.

## Validation Commands
- `make fmt`
- `make lint`
- `make vet`
- `make test`

## Expected Outcome
All functional requirements FR-001..FR-012 plus FR-004a..FR-004d and FR-009a are demonstrably satisfied with tests passing and no UX regressions in scroll behavior.