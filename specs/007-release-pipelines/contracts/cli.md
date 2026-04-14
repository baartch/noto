# CLI Contract: Version + Update Notice

## `noto version`

Outputs current version.

Example:
```
noto version
v1.2.3
```

Errors:
- none (falls back to `dev` if version unknown)

## Startup update notice (CLI)

When a newer version is available, print a non-blocking notice on startup.

Example:
```
Update available: v1.2.4 (current v1.2.3)
https://github.com/<org>/<repo>/releases/tag/v1.2.4
```

No notice when current version is latest or update check fails.