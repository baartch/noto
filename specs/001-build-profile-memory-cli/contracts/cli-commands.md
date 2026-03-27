# CLI Contract: Noto Commands and Interactive Behavior

## Startup Contract

1. If profile count = 0: force create-profile flow before entering chat.
2. If profile count = 1: auto-select profile and enter chat.
3. If profile count > 1: require profile/default selection before chat.

## Command Contract

### `noto profile create <name>`
- **Behavior**: Create profile, initialize prompt file, DB, and profile-local paths.
- **Success Output**: Created profile identifier + local path.
- **Failure Cases**: duplicate/invalid name, DB init failure, filesystem failure.

### `noto profile list`
- **Behavior**: List profiles with active/default indicators.

### `noto profile select <name>`
- **Behavior**: Set active profile for next chat and profile-scoped commands.
- **Failure Cases**: unknown profile.

### `noto profile rename <old> <new>`
- **Behavior**: Rename profile safely without cross-profile side effects.
- **Failure Cases**: unknown old profile, duplicate new name.

### `noto profile delete <name>`
- **Behavior**: Require explicit confirmation phrase; on confirm remove only target profile data.
- **Failure Cases**: confirmation mismatch/cancel, unknown profile, delete failures.

### `noto prompt show`
- **Behavior**: Display active profile prompt.

### `noto prompt edit`
- **Behavior**: Edit prompt; save; invalidate active profile context cache.

### `noto chat`
- **Behavior**: Enter interactive chat in active profile.
- **Runtime guarantees**:
  - Uses only active profile prompt/memory/cache/provider config.
  - On cache miss/invalid cache, rebuilds from persisted memory.
  - On session end, writes memory notes, summary, and session-end backup snapshot.

## Recovery Contract

- On profile DB corruption detection:
  1. Attempt automatic repair.
  2. If repair fails, restore latest valid profile-local backup snapshot.
  3. Emit clear user-visible notice of outcome and recovered point.
- Recovery process must never read/write unrelated profile data.

## Security Contract

- Provider credentials stored encrypted at rest.
- Commands/logs must never print decrypted credential material.
- Non-credential profile artifacts remain local files under OS permissions.

## Observability Contract

- Emit structured local logs and local metrics for:
  - startup flow decisions,
  - retrieval/cache hit/miss and invalidation,
  - provider call outcomes,
  - recovery attempts/results,
  - destructive command confirmations.

## Isolation Contract

- No command may read/write memory, prompt, cache, backups, credentials, or recovery state
  outside selected profile scope.
- Deleting/restoring one profile cannot alter other profiles.
