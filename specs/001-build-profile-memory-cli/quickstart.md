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
- Validate vector index corruption/missing-file handling triggers rebuild from SQLite.

## Vector Retrieval Checks

1. Populate profile with representative notes/messages.
2. Trigger embedding + vector index sync.
3. Run semantic retrieval query and verify top results map back to SQLite records.
4. Delete or tamper with vector index file and verify automatic rebuild.
5. Verify retrieval falls back safely when vector layer is unavailable.

## Performance Checks

- Benchmark slash suggestion refresh latency against p95 budget.
- Benchmark startup/context assembly/command feedback budgets.
- Benchmark vector top-k lookup latency against p95 budget.

## Scope Guardrails

- Out-of-scope: multi-user sync, cloud backup, vector memory as source-of-truth.
