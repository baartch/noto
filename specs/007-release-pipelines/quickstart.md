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

## PR traceability (FR-001–FR-009)

For every PR that changes this feature scope, add a Requirement → Test table in the PR description (or use `.github/pull_request_template.md`) and list only changed FRs with at least one automated test reference.

Minimum checklist:
- Map each changed FR to at least one test (unit/contract/integration)
- Include failure-path tests where applicable
- Keep table aligned with actual changed requirements in the PR