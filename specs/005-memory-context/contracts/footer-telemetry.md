# Contract: Footer Context Capacity Telemetry

## Purpose

Define how the chat footer reports token usage, model max context size, and current context usage percentage.

## Data Sources

- Latest normalized provider usage for the active chat model
- Active model metadata from the provider models API
- `context_length` from the OpenRouter Model Object Schema when available

## Footer Placement

The footer MUST display context-capacity telemetry on the left side next to the existing token status information.

## Required Fields

- **tokens_in_total**
- **tokens_out_total**
- **cost_usd**
- **context_used_tokens** for the most recent request
- **context_max_tokens** for the active model
- **context_used_percent** derived from `context_used_tokens / context_max_tokens`

## Behavior Rules

1. When `context_length` is known for the active model, the footer MUST show current percentage used and the max context size.
2. When `context_length` is unknown, the footer MUST keep token telemetry visible and show an explicit unknown-capacity state rather than a misleading percentage.
3. Context-capacity telemetry MUST update after provider responses as usage changes.
4. Switching models or profiles MUST update the displayed max-context value on the next successful metadata refresh or cached metadata lookup.
5. The footer format MUST remain compact enough to coexist with existing cache status and warning badges.

## Validation Targets

- Percentage is absent or replaced with explicit unknown state when max context is unavailable.
- Percentage and max update correctly after model switches.
- Footer remains readable with cache status, warnings, and profile/model labels.

## Traceability

- User input 2026-06-09: show max context size and used percentage in footer next to tokens
- OpenRouter models docs: `context_length` field in model object schema
