# Quickstart: Noto Profile Memory CLI

## Prerequisites

- Go 1.23+
- Local terminal on macOS/Linux
- Reachable model provider configuration

## Run

```bash
go run ./cmd/noto
```

## Slash Command Validation

1. Enter chat and type `/`.
2. Confirm command suggestions appear.
3. Type `/pro` and confirm profile commands are suggested.
4. Execute `/profile list` and verify behavior matches CLI mode.
5. Enter ambiguous slash input and confirm explicit selection is required.
6. Enter unknown slash input and confirm explicit error + top suggestions.
7. Confirm no suggestions appear during non-slash chat input.

## Reliability/Security Checks

- Validate encrypted credential handling.
- Validate corruption recovery (auto-repair then backup restore fallback).
- Validate no cross-profile effects from slash commands.

## Performance Checks

- Benchmark slash suggestion refresh latency against p95 budget.
- Benchmark startup/context assembly/command feedback budgets.

## Scope Guardrails

- Out-of-scope: multi-user sync, cloud backup, vector memory as source-of-truth.
