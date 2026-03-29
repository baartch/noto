# CLI and Chat Slash Command Contract

## Command Parity Contract

All CLI commands MUST be executable inside chat via slash format with equivalent behavior,
validation, and side effects.

## Canonical Slash Syntax

- Canonical format: `/group action [args...]`
- Examples:
  - `/profile list`
  - `/profile select "Career Coach"`
  - `/prompt show`

## Suggestion Contract

- Suggestions visible only when current input begins with `/`.
- Suggestions refresh on each keystroke in slash mode.
- Suggestions include command path + short hint.
- Ambiguous slash input requires explicit user selection; no auto-execution.
- Unknown slash command returns explicit error + top matching suggestions.

## Execution Safety Contract

- Destructive commands in slash mode require same explicit confirmation behavior as CLI mode.
- Slash command execution cannot bypass profile isolation checks.

## Existing Command Surface (applies to CLI + slash)

- `profile create <name>`
- `profile list`
- `profile select <name>`
- `profile rename <old> <new>`
- `profile delete <name>`
- `prompt show`
- `prompt edit`
- `chat`

## Recovery/Security/Isolation

- Recovery: auto-repair then backup restore fallback with user notice.
- Security: credentials encrypted at rest; no decrypted secrets in output/logs.
- Isolation: no cross-profile reads/writes from any command path.
