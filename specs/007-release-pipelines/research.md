# Research: Release Publishing & Update Checks

## GitHub Actions release pipeline

Decision: Use GitHub Actions with a release workflow triggered on tag push (`v*`) to build cross-platform binaries, run `make tidy fmt vet lint test`, then create GitHub Release with artifacts.
Rationale: Native to GitHub hosting, aligns with repo requirement, and supports multi-OS matrix builds and release publishing.
Alternatives considered: GoReleaser (richer features but extra dependency/config); manual release scripts (less reliable and repeatable).

## Versioning + update check implementation

Decision: Embed semantic version string at build time via ldflags (e.g., `-X noto/internal/version.Version=v1.2.3`) and use GitHub Releases API to check latest release tag. Compare versions using `golang.org/x/mod/semver`.
Rationale: Simple, no extra service; semver package handles ordering; GitHub Releases API provides public latest version.
Alternatives considered: custom version parser (more error-prone), using GitHub CLI (extra dependency).

## Non-blocking update notice patterns

Decision: Spawn background goroutine at startup with 1s timeout and report result via logger/UI notification channel for CLI/TUI.
Rationale: Meets NFR performance and non-blocking requirement; consistent with existing UX if routed through existing message surfaces.
Alternatives considered: synchronous check (violates NFR), scheduled cron check (out of scope).