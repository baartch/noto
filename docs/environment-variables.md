# Environment Variables

Noto reads the following environment variables.

## Provider

| Variable | Description |
|---|---|
| `NOTO_API_KEY` | API key for the AI provider. When set, overrides the stored provider config and skips the provider setup dialog. |
| `NOTO_ENDPOINT` | API endpoint URL. Used together with `NOTO_API_KEY`. Defaults to the provider's standard endpoint when empty. |
| `NOTO_MODEL` | Model ID to use. Used together with `NOTO_API_KEY`. |

## Paths

| Variable | Description |
|---|---|
| `NOTO_APP_DIR` | Override the application directory (`~/.noto` by default). Profiles, logs, and config are stored here. |

## Editor

| Variable | Description |
|---|---|
| `EDITOR` | External editor for editing prompts (`/prompt edit` or `Ctrl+E`). Falls back to `VISUAL`, then `vi`. |
| `VISUAL` | Fallback editor when `EDITOR` is not set. |

## Debugging

| Variable | Description |
|---|---|
| `DEBUG` | When set to any non-empty value, prints raw embedding API responses to stderr. |
