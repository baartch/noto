# Data Model: Release Publishing & Update Checks

## Release

Fields:
- version (string, semantic version, e.g., v1.2.3)
- notes (string, release notes markdown)
- published_at (timestamp)
- url (string, GitHub release URL)

Relationships:
- Release has many Artifacts

Validation:
- version must follow semantic versioning (MAJOR.MINOR.PATCH)

## Artifact

Fields:
- platform (enum: linux, windows, darwin)
- filename (string)
- url (string)
- checksum (string, optional)

Relationships:
- Artifact belongs to Release

## Version

Fields:
- current (string, semantic version)
- latest (string, semantic version, optional)

Validation:
- semantic version compare using `semver.Compare`

State transitions:
- latest: unknown -> known (after update check)
- latest: known -> unknown (on update check error)