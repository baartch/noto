# Quickstart: Messenger Chat UI History Scrolling

## Goal
Validate conversation history continuity, zone-aware scrolling, bounded lazy loading for conversation/input history, and manual FR-012 baseline verification.

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
   - Confirm startup history load errors (if any) are shown inline and do not block interaction.

3. **Messages-zone wheel routing check**
   - Move cursor over messages viewport.
   - Scroll wheel up/down and verify only conversation viewport changes.
   - Verify input text and input-history selection remain unchanged.

4. **Input-zone wheel routing + lazy load check**
   - Move cursor over input textarea.
   - First upward history scroll should load exactly 3 latest input-history entries.
   - Continue upward scrolling; verify additional loads happen in batches of 3.
   - Verify total loaded input-history entries never exceed 12.
   - Verify messages viewport does not move during input-zone scrolling.

5. **Page key routing check (global messages history)**
   - Hover cursor in messages area and press Page Up/Page Down; verify messages history scrolls.
   - Hover cursor in input area and press Page Up/Page Down; verify messages history still scrolls (input history must not be used for Page keys).

6. **Send reset check**
   - Submit an input.
   - Verify in-memory input-history window resets (next input-zone wheel scroll triggers fresh first-load of 3).

7. **Conversation lazy-load check**
   - In messages zone, scroll to top and continue scrolling up.
   - Verify older messages prepend in batches of 10.
   - Verify behavior is stable when no older messages remain (no crash/no extra fetch behavior).

8. **Profile switch check**
   - Switch profile.
   - Verify conversation window resets and loads new profile latest 10.
   - Verify input-history window resets to empty (on-demand reload policy preserved).

9. **Manual FR-012 baseline check**
   - Confirm user bubbles are right-aligned and assistant bubbles are left-aligned.
   - Confirm no layout regression from this feature’s scroll/history changes.

## Validation Commands
- `make fmt`
- `make lint`
- `make vet`
- `make test`

## Expected Outcome
All functional requirements FR-001..FR-012 plus FR-004a..FR-004d, FR-005a/FR-005b, and FR-009a are satisfied with tests passing and no UX regressions in scroll behavior.