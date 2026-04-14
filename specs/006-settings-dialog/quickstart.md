# Quickstart

## Preconditions
- Go 1.26+
- Local profile database available (created on first run)

## Run

```bash
cd /home/andy/gitrepos/noto
```

## Manual Test Checklist
1. Launch `noto`.
2. Press Ctrl+J to open settings.
3. Navigate to Profiles.
4. Confirm profiles list shows entries (active profile indicated).
5. Press Enter on a profile to switch.
6. Press Ctrl+N, enter a new profile name, press Enter to create.
7. Press Ctrl+R on a profile, rename, press Enter.
8. Press Ctrl+D to delete a profile (ensure not last).

## Performance Check
- Settings dialog opens in <1s with no visible lag.

## Build & Test Status

```
go test ./...  → passed
make lint      → passed
```
