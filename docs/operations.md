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
        ├── files/          # user-managed PDF, TXT, and MD reference files
        ├── prompts/
        │   └── system.md   # profile system prompt
        ├── cache/          # context cache artefacts (future)
        └── backups/        # timestamped DB and vector index snapshots
            ├── 20260101T120000Z.db
            └── 20260101T120000Z.vec
```

## Profile Reference Files

Put reference material directly under `~/.noto/profiles/<slug>/files/`; nested folders are included:

```text
~/.noto/profiles/<slug>/files/artist/identity.pdf
~/.noto/profiles/<slug>/files/press/voice.md
```

Supported formats are PDF with selectable text, TXT, and MD. Scanned PDFs that require OCR and image files are unsupported. Noto scans on startup and profile activation; there is no upload, live watcher, or manual refresh command. A newly added file is discovered on the next scan.

The profile's existing `memory.db` stores a separate reference catalog: relative path, type, content fingerprint, short description, status, and indexing metadata. Full document text is not persisted. A successful complete scan removes catalog rows for disappeared files; an inaccessible or interrupted scan does not treat unseen paths as deleted. Changed files are re-described, and chat-time reads recheck fingerprints. Unreadable files remain unavailable with a bounded diagnostic; one bad file does not prevent other files or ordinary chat.

Initial bounds: 16 MiB per source file, 4 MiB extracted text, 500 PDF pages, 2,048 bytes/200 words per description, 16 files selected per turn, and 32 KiB maximum combined reference text (also constrained to one quarter of the active model's context window and available prompt budget). Catalog selection is limited to 4 KiB. These byte-based admission limits are conservative estimates, not provider-exact tokenization. Over-limit sources/sets are reported as unavailable; text is not silently truncated. A document may be indexed successfully yet be too large to include in a chat request.

The configured extractor model generates descriptions and chooses relevant files; when no separate extractor model is set, the active chat model is used. Indexing and selection calls consume provider tokens/cost. Original file text is sent to the configured provider for indexing and selected chat turns, so apply the same provider trust and privacy expectations as ordinary chat. Noto puts temporary attachments after system/memory/catalog/history and before the current user message to preserve a stable provider-cache prefix when possible. Cache hits, TTL, and retention are provider-dependent; removing an attachment from later requests does not guarantee deletion from provider caches or logs.

Loaded text is held only in the current session and cleared on a complete topic change, restart, and profile switch. Saved user/assistant replies remain ordinary conversation history. When file summaries or file text are in context, Noto extracts memory candidates from the user's message only; assistant replies are not automatically turned into memory notes. This conservative policy avoids indirect capture of file-derived facts.

### PDF reader license

The PDF text reader is `github.com/ledongthuc/pdf` at commit `6c8c28e0e8a07a3452f9d8cf3090ed6e856d8f24`, licensed under the BSD 3-Clause License:

```text
Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. Neither the name of Google Inc. nor the names of its contributors may be
   used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
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
