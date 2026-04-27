# Contract: TUI Flow Standardization and Footer Telemetry

**Feature**: [Bubble Tea TUI Standard](../spec.md)

This contract defines required behavior for refactored TUI flows, component reuse, and footer telemetry.

## Requirements

1. Every TUI flow is implemented as a Bubble Tea model with update/view loops.
2. Navigation keys, labels, and visual styles remain consistent with existing UI unless explicitly documented.
3. If a suitable Bubbles component exists, it must be used.
4. If a custom component is used, its rationale is documented in-code or in supporting docs.
   - Current rationale: no broad replacement candidates remain; existing picker/help/list surfaces already use Bubbles primitives. `internal/tui/components.go` documents shared composition conventions.
5. Styling is defined via reusable Lip Gloss styles.
6. Footer always renders: token usage (`up/down/cache read/cache write`), context cache stats (`ctx:miss|hit`), total cost, current profile, current main model, app version, and help keybinding.
7. Token/cost totals are extracted from provider `usage` payloads when present.
8. Token/cost totals include contributions from main chat model, extractor model, and embeddings model operations.
9. Responses without `usage` payload do not mutate accumulated totals.

## Usage Payload Mapping

Given a provider response usage object:

- `completion_tokens` → `down`
- `prompt_tokens` → `up`
- `prompt_tokens_details.cached_tokens` → `cache read`
- `prompt_tokens_details.cache_write_tokens` → `cache write`
- `cost` → `cost`

## Validation

- Each flow has an explicit entry point and can be triggered in the TUI.
- Each flow lists Bubbles usage or custom rationale.
- Footer remains anchored and visible across normal TUI states and overlays.
- Footer values reflect accumulated usage across all required model classes.
- Refactored flows continue to pass integration and contract tests.
