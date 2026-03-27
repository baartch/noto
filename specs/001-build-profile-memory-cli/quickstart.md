# Quickstart: Noto Profile Memory CLI

## Prerequisites

- Go 1.23+
- Local terminal on macOS/Linux
- At least one reachable provider endpoint (GitHub Models, OpenRouter, local OpenAI-compatible,
  or generic OpenAI-compatible)

## Run

```bash
go run ./cmd/noto
```

## First-Run Validation

1. Start app.
2. Confirm zero-profile flow forces profile creation.
3. Configure provider credentials and verify credentials are stored encrypted.
4. Start chat; end session; confirm summary/notes and session-end backup are written.
5. Restart app; confirm continuity context loads (cache hit path if available).

## Core Commands

```bash
noto profile create "Career Coach"
noto profile list
noto profile select "Career Coach"
noto prompt show
noto prompt edit
noto chat
noto profile rename "Career Coach" "Exec Coach"
noto profile delete "Exec Coach"
```

## Reliability and Recovery Checks

- Simulate cache corruption: verify invalidation + rebuild from memory.
- Simulate DB corruption: verify auto-repair attempt, then backup restore fallback.
- Verify recovery notice explains outcome and recovery point.

## Observability Checks

- Verify structured log entries for startup, retrieval, cache, provider error, and recovery paths.
- Verify profile-scoped local metrics update for the same flows.

## Performance Checks

- Benchmark startup (warm) against plan budget.
- Benchmark first contextual response for cache-hit and cache-miss paths.
- Verify command feedback latency for profile operations.

## Scope Guardrails

- Out-of-scope for this release: multi-user sync, cloud backup, vector memory as source of truth.
