# Noto

Noto is a chatbot that creates notes—locally on your system. These notes are available at any time, and the more you chat with Noto, the better he gets to know you and the topics that interest you. The idea behind Noto’s development is to use it as a coach and mentor. It helps you develop strategies and reminds you of relevant things. Since it takes notes about you and your progress (whatever topic you’re currently working on), it has a much more comprehensive understanding of the progress you’re making.

Noto has a WebGUI:
<img width="2666" height="1824" alt="127 0 0 1_8080_" src="https://github.com/user-attachments/assets/efcaf50c-6183-475d-a969-e86240c20e2f" />
 
And Noto has a TUI:
<img width="1142" height="799" alt="Screenshot_2026-10-20_20-26-41" src="https://github.com/user-attachments/assets/58240bf5-e0aa-47c4-bf56-3d29e74f79f6" />


## Getting started

### 1) Create a profile

```bash
noto profile create "My Profile"
noto profile select "My Profile"
```

### 2) Configure a provider

```bash
noto provider set \
  --endpoint https://openrouter.ai/api/v1 \
  --key <YOUR_OPENROUTER_API_KEY> \
  --model openai/gpt-4o-mini
```

### 3) Select an embeddings model

In Settings (ctrl+j), pick **Model Embeddings** to enable memory retrieval.

Then start a chat session:

```bash
noto
```

Or start the browser workspace:

```bash
noto web
```

Open the printed `http://127.0.0.1:8080` address. The web UI uses the active profile's existing provider, conversation history, notes, and weekly/monthly summaries. Provider setup must be completed in the terminal first. See the [web GUI operations notes](docs/operations.md#web-gui) for localhost, lifecycle, and build details.

Web chat also supports optional post-recording speech dictation. Configure it in **Settings → Voice** for each profile. Local Parakeet is an explicit, shared model download; provider transcription uses a separately selected verified speech-to-text model and may incur provider charges. See the [voice dictation guide](docs/voice-dictation.md) for privacy, setup, keyboard controls, costs, and platform limits.

## Web UI development

The browser source is in `internal/web/ui`; `make web-build` bundles it into `internal/web/static`, which the Go binary embeds. Rebuilding the UI requires Bun and the project's pinned JavaScript dependencies. On a fresh checkout, install them with `cd internal/web/ui && bun install --frozen-lockfile`. Tailwind CSS is built with the project-local `@tailwindcss/cli`; no global Tailwind installation is needed.

```bash
make web-build          # Bun bundle and local Tailwind CSS build
make web-test           # Bun UI state, interaction, and voice-controller tests
make web-browser-test   # Playwright tests in desktop and 390×844 mobile Chrome
make build              # Go binary with the generated web assets embedded
```

`make web-test` requires Bun. `make web-browser-test` also requires Node.js, the pinned project-local Playwright package, and an installed Chrome browser. Bun's `browser` build target selects browser platform resolution but does **not** down-convert JavaScript syntax to ES2022; the generated bundle is tested in Chrome, and no ES2022 compatibility floor is claimed. At runtime the Go binary serves the embedded UI without Bun or Node and makes no third-party asset requests; chat still uses the provider configured for the active profile.

## Profile reference files

Place reference documents directly in a profile's `files/` directory; there is no upload command:

```text
~/.noto/profiles/<profile>/files/artist/identity.pdf
~/.noto/profiles/<profile>/files/press/voice.md
```

Noto indexes **PDF (selectable text only), TXT, and MD** files on startup and profile changes, then chooses relevant files automatically for each chat turn. Scanned PDFs/OCR and images are not supported. Nested directories are scanned; symlinks are ignored. New files become available after the next startup or profile activation.

File contents are sent to your configured provider when indexed and when selected for a chat turn. Only a short file description is stored in the profile database; loaded text is temporary and is cleared on topic changes, restarts, and profile switches. Chat replies remain in history, but replies influenced by reference files are not automatically saved as memory notes. Provider-side prompt caching depends on the provider and does not guarantee deletion from its retention systems.

See the [operations runbook](docs/operations.md#profile-reference-files) for limits and failure behavior.

## Version and updates

- Print current version:

```bash
noto version
```

- On startup, noto performs a non-blocking update check against GitHub Releases.
- In TUI mode, the current version is shown in the footer.
