# Noto

Noto is a local-first terminal chatbot with profile-scoped memory and a consistent TUI experience.

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
