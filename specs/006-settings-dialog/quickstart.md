# Quickstart: Settings Profile Management

## Build & Test Status

```
go test ./...  → all packages pass
make lint      → 0 issues
```

## Profile Management Flow (Settings)

1. Press **Ctrl+J** to open Settings.
2. Navigate to **Profiles** and press **Enter**.
3. Choose an action:
   - **Select**: pick a profile; active profile switches.
   - **Create**: enter a new profile name.
   - **Rename**: enter old + new name.
   - **Delete**: confirm deletion.
4. **Esc** returns to parent menu; **Esc** at root closes Settings.

## Performance

Profile submenu opens in <1s (manual check).