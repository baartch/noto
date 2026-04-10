# Research: Bubble Tea TUI Standard

**Date**: 2026-04-10

## Current State

- The repository already uses Bubble Tea and Bubbles for the main TUI model (`internal/tui/model.go`).
- Several UI elements (suggestions, pickers, viewport) are implemented with Bubble Tea/Bubbles patterns.
- The remaining work is to inventory any TUI flows not currently implemented as Bubble Tea models or that use custom UI where a Bubbles component might suffice.
- Styling is currently handled via inline styles; these should be consolidated into reusable Lip Gloss definitions where applicable.

## Decision

### Decision: Refactor all TUI surfaces to Bubble Tea + Bubbles + Lip Gloss

- **Chosen**: All existing TUI flows will be migrated to Bubble Tea models/update loops.
- **Chosen**: Bubbles components will be used wherever a suitable component exists; custom components must document rationale.
- **Chosen**: Styling definitions will be consolidated into reusable Lip Gloss styles.

## Rationale

- Enforces a consistent architectural approach for all TUI behavior.
- Reuses proven components to reduce maintenance and UX drift.
- Centralized Lip Gloss styles reduce visual inconsistency and simplify future changes.

## Alternatives Considered

1. **Apply only to new TUI work**
   - Rejected: Requirement explicitly mandates refactoring existing TUI flows.

2. **Allow custom components without documentation**
   - Rejected: Increases maintenance cost and inconsistency risk.

3. **Leave styling inline per component**
   - Rejected: Makes it harder to enforce a consistent look and feel across the app.

## Notes / Constraints

- Inventory effort required to identify all TUI flows and map them to Bubble Tea/Bubbles components.
- UX behavior should remain consistent with existing patterns unless explicitly documented.
- Lip Gloss styles should be defined in reusable blocks to prevent drift.
