# Contract: TUI Flow Standardization

**Feature**: [Bubble Tea TUI Standard](../spec.md)

This contract defines expectations for refactored TUI flows and styling.

## Requirements

1. Every TUI flow is implemented as a Bubble Tea model with update/view loops.
2. Navigation keys, labels, and visual styles remain consistent with existing UI unless explicitly documented.
3. If a suitable Bubbles component exists, it must be used.
4. If a custom component is used, its rationale is documented in-code or in supporting docs.
5. Styling is defined via reusable Lip Gloss styles.

## Validation

- Each flow has an explicit entry point and can be triggered in the TUI.
- Each flow lists its Bubbles component usage or custom rationale.
- Each flow references the Lip Gloss styles it uses.
- Refactored flows continue to pass existing integration tests.
