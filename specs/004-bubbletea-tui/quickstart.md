# Quickstart: Bubble Tea TUI Standard

**Goal**: Validate Bubble Tea/Bubbles/Lip Gloss standardization and verify always-visible footer telemetry with cross-model usage accounting.

## Prerequisites

- Buildable `noto` workspace.
- Test environment able to run unit, integration, and contract suites.
- Ability to trigger flows that call main chat, extractor, and embeddings operations.

## Verification Steps

1. **Run quality gates**
   - `make fmt`
   - `make lint`
   - `make vet`
   - `make test`

2. **Check Bubble Tea model coverage**
   - Inventory key TUI flows (startup, model picker, settings/help, suggestions/overlays).
   - Confirm each flow uses Bubble Tea update/view patterns.

3. **Check Bubbles and Lip Gloss reuse**
   - Confirm suitable Bubbles components are used.
   - Confirm styles are reusable Lip Gloss definitions, not ad-hoc inline drift.

4. **Check footer always-visible fields**
   - In normal view and while overlays are active, verify footer includes:
     - `up/down/cache read/cache write/cost`
     - `ctx:miss|hit`
     - current profile
     - current main model
     - app version
     - help keybinding

5. **Check usage extraction mapping**
   - Use mocked/provider-captured responses with `usage` payloads.
   - Verify mapping:
     - `completion_tokens -> down`
     - `prompt_tokens -> up`
     - `prompt_tokens_details.cached_tokens -> cache read`
     - `prompt_tokens_details.cache_write_tokens -> cache write`
     - `cost -> cost`

6. **Check cross-model accumulation**
   - Trigger main chat request(s), extractor request(s), embeddings request(s).
   - Verify footer totals reflect sum across all three model classes.

7. **Check missing-usage behavior**
   - Simulate response without `usage`.
   - Verify totals remain unchanged (no reset/no estimation).

8. **Check settings shortcut parity**
   - Press `Ctrl+J` in active TUI.
   - Verify settings dialog opens and footer remains visible.

## Expected Results

- All TUI flows align with Bubble Tea architecture and Bubbles-first component selection.
- Footer remains anchored and always visible with required telemetry/status fields.
- Usage/cost values are correctly parsed from response payloads and aggregated across main, extractor, and embeddings models.
- Quality and test gates pass with no regressions.
