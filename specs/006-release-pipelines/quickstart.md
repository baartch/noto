# Quickstart: Release Publishing & Update Checks

## Release pipeline (GitHub Actions)

1. Update version (tag `vX.Y.Z`).
2. Push tag to GitHub.
3. Workflow runs `make tidy fmt vet lint test`.
4. Workflow builds Linux/Windows/macOS binaries, uploads artifacts, and publishes GitHub Release.

## Verify update notice

1. Set current version to `vX.Y.Z`.
2. Create a newer GitHub Release (e.g., `vX.Y.(Z+1)`).
3. Start `noto` in CLI and TUI; observe non-blocking update notice.
4. Disable network; verify startup continues with no notice.