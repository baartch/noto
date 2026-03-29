# Noto Operations Runbook

## Overview

Noto stores all persistent data in `~/.noto/`. Each profile has its own isolated directory
under `~/.noto/profiles/<slug>/`.

## Directory Structure

```
~/.noto/
├── active_profile          # text file containing the active profile slug
└── profiles/
    └── <slug>/
        ├── memory.db       # per-profile SQLite source-of-truth
        ├── memory.vec      # per-profile vector index (derived, rebuildable)
        ├── prompts/
        │   └── system.md   # profile system prompt
        ├── cache/          # context cache artefacts (future)
        └── backups/        # timestamped DB and vector index snapshots
            ├── 20260101T120000Z.db
            └── 20260101T120000Z.vec
```

## Backup & Restore

### Manual snapshot

```go
// Go API
backup.Snapshot("my-profile-slug")
```

```bash
# Or copy manually:
cp ~/.noto/profiles/<slug>/memory.db ~/.noto/profiles/<slug>/backups/$(date -u +%Y%m%dT%H%M%SZ).db
```

### Restore from latest backup

```go
backup.Restore("my-profile-slug")
```

Backups are pruned to the last 10 snapshots automatically.

## Vector Index Rebuild

If the vector index (`memory.vec`) is corrupted, missing, or stale, rebuild it from SQLite:

```go
// Programmatic rebuild
rebuilder := vector.NewRebuilder(manifestRepo, vectorIndex, profileSlug)
rebuilder.Rebuild(ctx, notes)
```

The vector index is always reconstructable from `memory.db`; SQLite is the source of truth.

## Corruption Recovery

The `backup.Recover(slug, w)` function performs:

1. Checks if `memory.db` exists.
2. If missing → attempts to restore from the latest backup.
3. If present but failed integrity check → auto-repair or backup restore.

Recovery is logged to the provided `io.Writer` with a user-visible notice.

## Performance Budgets

| Operation                  | p95 target   |
|----------------------------|--------------|
| Startup (warm)             | < 1.5s       |
| First contextual response  | < 700ms (cache hit) / < 2.0s (miss) |
| Profile command feedback   | < 150ms      |
| Slash suggestion refresh   | < 50ms/keystroke |
| Vector top-k lookup        | < 40ms       |

Run benchmarks with:

```bash
make bench
```

## Provider Credentials

Credentials are encrypted at rest using AES-256-GCM. The `security` package provides
`Encrypt` / `Decrypt` helpers. **Never log or output decrypted credentials.**

## Profile Isolation

Each profile's `memory.db` contains only that profile's data. The `store.IsolationGuards`
type provides runtime checks. Cross-profile reads/writes are explicitly rejected.

## CLI Quick Reference

```bash
noto profile create <name>      # create a new profile
noto profile list               # list all profiles (* = active)
noto profile select <name>      # set active profile
noto profile rename <old> <new> # rename a profile
noto profile delete <name>      # delete a profile (requires confirmation)
noto prompt show                # show active profile system prompt
noto prompt edit                # edit system prompt in $EDITOR
noto chat                       # start interactive chat session
```

## Slash Command Reference (in chat)

```
/profile create <name>
/profile list
/profile select <name>
/profile rename <old> <new>
/profile delete <name>
/prompt show
/prompt edit
/model extractor <model>
```

Type `/` to see all available commands. Type `/pro` to see profile-related commands.
