# Web Voice Dictation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for inline execution, or superpowers:subagent-driven-development if the user explicitly chooses delegation. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Preline-styled web dictation with selectable local Parakeet v3 or profile-provider speech-to-text.

**Architecture:** The browser records bounded mono WAV, then requests transcription through the same-origin Go service and reuses the existing chat submission path. Profile-local voice settings select a shared, explicitly installed local model or an OpenRouter transcription model; a provider-neutral speech interface separates inference from UI and routing.

**Tech Stack:** Go 1.27 (the module/CI target; repository guidance says 1.26+), SQLite, sherpa-onnx Go bindings/native runtime (subject to Task 1), Bun, vanilla JavaScript, Playwright, Preline/Tailwind.

**Spec:** `docs/superpowers/specs/2026-11-22-web-voice-dictation-design.md`

## Global Constraints

- German is primary; English must work; no live transcript, TUI voice, TTS or automatic model download.
- Idle input placeholder: `Type your message…`; use the linked Preline microphone and paper-plane buttons and matching recording overlay.
- X/Escape cancels; period/pencil transcribes to draft; Enter/paper-plane transcribes, appends and sends once.
- Speech selection is per profile; the Parakeet model is shared. No silent cloud fallback.
- Reuse the profile provider endpoint and credentials; OpenRouter model discovery must use `output_modalities=transcription`.
- Protect credentials and raw audio; same-origin checks remain in force, and microphone permission is restricted to self.
- Do not commit or delegate without explicit user authorization. After dependency changes run `make tidy`; after code changes run `make fmt lint test`.

## Review Focus

1. Profile changes while transcription is in flight → never send to the new profile or overwrite its draft (Tasks 5, 7).
2. Duplicate stop/Enter events or retry after failure → at most one message, preserving existing idempotency (Task 7).
3. Interrupted downloads, malicious archives, delete during inference → no partial ready state, traversal or unsafe deletion (Task 4).
4. Malformed/oversized WAV, denied microphone and no audio → no message sent, draft and server protected (Tasks 5, 7).
5. Provider returns audio-capable chat models or stale metadata → show only verified transcription IDs, never silently select another (Tasks 2, 6).

## File Structure

- `internal/store/migrations/profile/0006_voice_settings.sql`, `internal/store/voice_settings_repo.go`: per-profile voice settings.
- `internal/voice/`: speech contract, WAV validation, OpenRouter discovery/provider, local model lifecycle, Linux/amd64 sherpa adapter and unsupported-platform stub.
- `internal/app/web_voice.go`, `internal/app/web_cmd.go`: connect profile provider credentials, store and voice services to web startup.
- `internal/web/voice.go`, `internal/web/server.go`, `internal/web/settings.go`: voice endpoints, settings coordination and same-origin security.
- `internal/web/ui/voice.js`, `internal/web/ui/voice.test.js`: browser capture, state and transcript actions.
- `internal/web/ui/index.html`, `app.js`, `input.css`, `browser/fixture-server.js`, `browser/web.spec.js`: settings, composer and browser tests.
- `internal/web/static/*`: rebuilt embedded web assets; update `Makefile`, `README.md` and release/build documentation as needed.

---

### Task 1: Validate local runtime and artifact distribution

**Files:** Create `docs/superpowers/voice-runtime-notes.md`; modify build/release files only if required by the verified integration.

**Interfaces:** Produces a pinned candidate sherpa-onnx binding/runtime version, upstream OS/architecture matrix, artifact URL, license, download/unpacked size estimates, and a tested or explicitly blocked build/packaging approach for Task 4. Seek trusted archive or per-file hashes; if the archive publisher does not publish a digest, document an approved pinned per-file integrity manifest rather than inventing an archive digest.

- [x] Inspect the project's release/build configurations and sherpa-onnx Go bindings. Verify the Parakeet v3 int8 artifact and model license from primary sources. Paseo lists `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8` at 487,170,055 download bytes; verify rather than assuming the installed size is the same. **Result:** candidate binding pin is sherpa-onnx-go/v2 v2.0.3; official release HEAD is 487,211,443 bytes; NVIDIA model card confirms CC BY 4.0. GitHub has no archive digest; the Sherpa Hugging Face export has a pinned per-file manifest, see runtime notes.
- [x] Build and run a disposable, opt-in local inference probe using short German and English recordings on an available supported platform. Ask the user before downloading the large model for this probe; use no private audio without consent. Record exact commands and observed result in the notes. If permission, recordings or platform are unavailable, mark the real-model check blocked, not passed. **Probe blocked (user declined download); local inference remains unverified.**
- [x] Verify a normal build without optional native assets and document the release packaging/build-tag approach for each supported platform. If a platform cannot support local inference, ensure the eventual UI reports unsupported; do not claim full support or silently replace local inference with cloud. **Result:** baseline `make build` passes without speech dependencies. Upstream platform matrix and a Linux/amd64+cgo+!musl adapter/stub proposal are recorded; package linking and release-library packaging are validated in Task 4.
- [x] Run `go test ./internal/store ./internal/web` as a baseline. Expected: PASS. Do not proceed with a guessed runtime or checksum; report any unresolved packaging blocker. **Result: PASS; no archive digest is available, but Task 4 now uses approved pinned Hugging Face per-file hashes for extracted runtime files.**

### Task 2: Profile settings and OpenRouter discovery

**Files:** Create `internal/store/migrations/profile/0006_voice_settings.sql`, `internal/store/voice_settings_repo.go`, `internal/store/voice_settings_repo_test.go`, `internal/voice/provider_models.go`, `internal/voice/provider_models_test.go`.

**Interfaces:** Produces `store.VoiceSettings{Provider string, Model string}`, `(*store.VoiceSettingsRepo).Get(ctx context.Context) (store.VoiceSettings, error)`, `(*store.VoiceSettingsRepo).Save(ctx context.Context, settings store.VoiceSettings) error`, and `voice.ListTranscriptionModels(ctx context.Context, cfg provider.Config) ([]string, error)`.

- [x] Write store tests asserting a migrated existing profile defaults to `{Provider:"", Model:""}`, two profile databases retain independent settings, and invalid providers are rejected. Write discovery tests using `httptest`: OpenRouter receives `/models?output_modalities=transcription`; responses with `architecture.output_modalities` of `transcription` are returned and audio-to-text chat models are excluded; errors and unsupported non-OpenRouter discovery are distinct. **Result:** existing-profile migration, isolated profile settings, metadata filtering, hostile hosts, response limits, cancellation, and sanitized errors covered.
- [x] Run `go test ./internal/store ./internal/voice -run 'VoiceSettings|TranscriptionModels'`; expect missing package/symbol failures. **RED observed:** expected missing repo/discovery symbols.
- [x] Implement the singleton profile table and repository. Normalize endpoint paths consistently with `internal/provider/models.go`, filter and deduplicate returned transcription model IDs, use timeouts and avoid logging credentials. Use the configured profile endpoint/key only; recognize OpenRouter by parsed hostname `openrouter.ai`, not string prefix or attacker-controlled suffix. **Result:** `provider.IsOpenRouterEndpoint` is shared by chat payload classification and voice discovery; requests do not follow redirects.
- [x] Run the same command; expect PASS. Run `go test ./internal/store ./internal/voice`; expect PASS. **Result:** focused tests, package tests, provider tests, `go vet`, `make lint` (0 issues), and `make test` pass.

### Task 3: Provider transcription adapter

**Files:** Create `internal/voice/speech.go`, `internal/voice/wav.go`, `internal/voice/openai.go`, `internal/voice/openai_test.go`, `internal/voice/wav_test.go`.

**Interfaces:** Produces `type Transcriber interface { Transcribe(context.Context, []byte) (string, error) }`, `voice.ValidateWAV(data []byte) error`, and `voice.NewProviderTranscriber(cfg provider.Config, model string) voice.Transcriber`.

- [x] Write tests asserting WAV validator accepts mono PCM16 at 16 kHz and the exact five-minute size; rejects truncated headers, inconsistent/overrun chunks, stereo, empty/unaligned data, and one byte over the size limit. Enforce the canonical 44-byte WAV header by rejecting extra `LIST`/extended/duplicate chunks. Provider tests cover multipart `model`/`file`, `audio/wav`, normalized endpoint variants, loopback HTTP vs remote HTTPS, redirect refusal, response bounds, 401/403, malformed/network error redaction and cancellation. Do not force English language. **Result:** boundary and failure-path tests cover all listed cases.
- [x] Run `go test ./internal/voice -run 'WAV|ProviderTranscriber'`; expect missing-symbol failures. **RED observed:** missing validator/transcriber symbols.
- [x] Implement validation with a 9,600,044-byte maximum for five minutes of canonical mono 16 kHz PCM16 RIFF/WAVE. Implement multipart upload with an `audio/wav` file part, response limits, cancellation and timeout; require HTTPS for remote provider endpoints (allow loopback HTTP for local services), do not follow redirects or log/return provider error bodies containing secrets or audio. Preserve language auto-detection.
- [x] Run the same command; expect PASS. **Result:** focused and package tests pass; `go vet`, `make lint` (0 issues), and `make test` pass.

### Task 4: Local model lifecycle and inference

**Files:** Create `internal/voice/local_models.go`, `internal/voice/local_models_test.go`, `internal/voice/local_sherpa_linux_amd64.go`, `internal/voice/local_sherpa_linux_amd64_test.go`, `internal/voice/local_stub.go`, `internal/voice/local_stub_test.go`, `internal/voice/local_test.go`; update `go.mod`, `go.sum` and `.github/workflows/release.yml` based on Task 1; update `docs/superpowers/voice-runtime-notes.md`.

**Interfaces:** Produces `voice.ModelManager` with `Status() ModelStatus`, `Download(ctx context.Context) error`, `Delete(ctx context.Context) error`, and `LocalTranscriber() (Transcriber, error)`; `ModelStatus` exposes runtime availability, installed/verified/verifying/downloading/deleting state, progress, sizes and a sanitized error reason.

- [x] Write tests with a local HTTP fixture and fake recognizer: no startup hashing or network before use/download; explicit download progress and per-file checksum failure; interrupted download cleans staging; reject traversal/symlinks and unsafe PAX metadata while ignoring bounded regular docs/test fixtures; repeated downloads are idempotent and delete during inference cannot corrupt active use; installed model shared across profiles; unsupported runtime reports unavailable. Verify local transcriptions remain local-only. **Result:** regression coverage also caught invalid-install repair, failed-delete state, corrupt same-size files, and cancellation while queued behind inference.
- [x] Run `go test ./internal/voice -run 'ModelManager|LocalTranscriber'`; expect missing-symbol failures. **Result:** initial run failed on missing manager/transcriber symbols as expected.
- [x] Resolve Task 1's artifact-integrity gate before enabling the production download. **Result:** user approved the pinned Sherpa Hugging Face per-file SHA-256 manifest at commit `ed5d745623472f97cfc2e6c2916ee248798fc5a8`; GitHub archive has no hash, so treat it only as transport and verify every required extracted runtime file against the manifest. User approved downloading only the 93,939-byte `tokens.txt` during development to compute its SHA-256; do not download ONNX weights or the archive for development tests.
- [x] Implement model install under `~/.noto/models/local-speech/parakeet-tdt-0.6b-v3-int8` using the approved per-file manifest, bounded safe extraction, atomic install, synchronized inference/delete and partial cleanup. Implement sherpa CPU inference with German/English auto-detection and no audio persistence. Supply an unsupported-platform stub with the same interface. Show verified download and installed sizes. **Result:** startup checks types/sizes and defers SHA-256 until first use; queued inference/install/delete honors context cancellation; corrupt installs are replaced only after the staged files pass the pinned hashes, and failed promotion clears installed/verified readiness.
- [x] Run `make tidy` after dependency changes, then `go test ./internal/voice`; expect PASS. Execute and record the opt-in real-model checks from Task 1 where prerequisites are available; explicitly report skipped checks. **Result:** `make tidy`, `make fmt lint test`, `make build`, `make vet`, race, cgo-disabled, Windows/macOS cross-compile and musl-tag checks pass. Real Parakeet inference and empirical GitHub archive-layout validation remain skipped because the user declined downloading the archive/weights; extracted files still fail closed unless they match the pinned per-file manifest. The release workflow runs the exact `TestSherpaRuntimeLoadsWithoutModelFiles` test and requires sherpa/ONNX libraries to resolve from `dist/lib`, preventing module-cache RUNPATH fallback; CI itself has not run here.

### Task 5: Web voice API and security

**Files:** Create `internal/web/voice.go`, `internal/web/voice_test.go`, `internal/app/web_voice.go`, `internal/app/web_voice_test.go`; modify `internal/web/server.go`, `internal/app/web_cmd.go`, `internal/web/ui/browser/fixture-server.js`. Voice-specific Service coordination lives in `voice.go`; existing `internal/web/settings.go` types are sufficient without edits.

**Interfaces:** Produces `web.VoiceBackend` with `Get(ctx, profileSlug) (VoiceDTO, error)`, `Save(ctx, profileSlug, settings, revision) error`, `Models(ctx, profileSlug) ([]string, error)`, `Transcribe(ctx, profileSlug, wav) (string, error)`, `Download(ctx) error`, `Delete(ctx) error`, and `ModelStatus() voice.ModelStatus`; exact Go parameter and return types are `context.Context`, `string`, `store.VoiceSettings`, `uint64`, `[]byte`, and the DTO described below. `VoiceDTO` contains profile slug, saved selection, readiness, model status and sanitized reason, never a key.

- [x] Write handler tests for `/api/voice`, `/api/voice/models`, `/api/voice/transcriptions`, `/api/voice/local-model`, `/api/voice/local-model/download`: enforce methods, origin/host, 9,600,044-byte WAV cap and correct MIME, unavailable/unsupported results, redacted errors and revision/profile mismatch. A blank or backend/model-mismatched selection must remain not-ready and must never dispatch to a different backend. Verify requests cannot dispatch a stale profile's audio to a newly active profile and delete/download conflicts are reported. **Result:** web tests cover exact five-minute WAV boundaries, route security, active-profile rejection, stale-revision snapshots and redaction; app tests cover per-profile readiness, verified-model routing, unsupported providers and shutdown-cancellable downloads.
- [x] Run `go test ./internal/web ./internal/app -run 'Voice|Microphone'`; expect failure before implementation. **RED observed:** failed on missing `VoiceDTO`, `NewHandlerWithVoice`, `webVoiceBackend` and `voice.LocalModelID` symbols.
- [x] Implement GET/POST voice settings, GET models, POST WAV transcription with `profile_slug` supplied separately from raw bytes, GET/DELETE local-model status and POST download. Because current host/origin guard applies to POST only, explicitly enforce matching Origin for DELETE as well. Bind the backend at startup, use existing runtime/revision coordination, and avoid keeping a closed profile DB across switches. Return download progress through status polling; download survives request completion but is cancelled on server shutdown. **Result:** raw WAV uses a separate `profile_slug` query; saves require active profile and current revision; settings/model reads carry a revision snapshot; DB handles are opened/closed per profile operation; transcription holds an active-profile lease; background download uses server lifetime context.
- [x] Change production and browser-fixture `Permissions-Policy` from `microphone=()` to `microphone=(self)` while keeping camera/geolocation disabled and the existing CSP. Verify the existing fixture parity test and same-origin security tests.
- [x] Run `go test ./internal/web ./internal/app`; expect PASS. **Result:** both packages pass; `make fmt lint test`, build, vet, race, CGO-disabled, Windows/macOS cross-compile and musl-tag checks pass.

### Task 6: Voice settings page

**Files:** Modify `internal/web/ui/index.html`, `internal/web/ui/app.js`, `internal/web/ui/browser/fixture-server.js`, `internal/web/ui/browser/web.spec.js`; add helpers/tests to `internal/web/ui/voice.js` and `internal/web/ui/voice.test.js`.

**Interfaces:** Consumes Task 5 voice endpoints. Produces `voiceReady(voiceDTO) -> boolean` and `filterAvailableVoiceModels(models, search) -> string[]`; settings UI uses existing profile draft/revision and settings save conventions.

- [x] Write Bun tests for `voiceReady` (unconfigured, local missing, unsupported runtime, installed local, configured provider) and filtering. Write Playwright tests for the separate Voice tab, explicit download/progress/delete confirmation, provider list restricted to returned STT IDs, empty/error/retry, and a profile switch with late model response.
- [x] Run `cd internal/web/ui && bun test voice.test.js`; expect failure before helpers exist. Run the focused Playwright tests after fixture stubs are added; expect missing UI failures. **RED observed:** Bun first failed on missing `voice.js`; focused Playwright tests failed because the Voice tab did not exist.
- [x] Implement the Voice tab and controls with local model details and explicit install/delete. Use the active profile provider model list; no chat-model mixing or silent fallback. Coordinate save/revision with the existing settings form, distinguish draft from persisted settings, and communicate shared-model deletion effects. **Result:** the Settings dialog has a Voice tab, draft-safe per-profile selection, STT-only model search/retry, shared local download progress/delete confirmation, and explicit readiness/unavailable states.
- [x] Run `cd internal/web/ui && bun test voice.test.js`, `make web-build`, and `make web-browser-test`; expect PASS. **Result:** Bun voice tests pass (7/7); full browser suite passes (34 passed, 2 existing platform skips). Generated `internal/web/static/*` outputs are intentionally committed in Task 8, which owns embedded assets.

### Task 7: Preline composer and recording flow

**Files:** Modify `internal/web/ui/index.html`, `internal/web/ui/app.js`, `internal/web/ui/voice.js`, `internal/web/ui/voice.test.js`, `internal/web/ui/input.css`, `internal/web/ui/browser/fixture-server.js`, `internal/web/ui/browser/web.spec.js`.

**Interfaces:** Consumes Task 5 voice APIs and Task 6 readiness. Produces `createVoiceController({ getUserMedia, audioContextFactory, transcribe, onInsert, onSend, onState })` with `start(profileSlug)`, `finish(action)`, `cancel()` and `dispose()`; `action` is `"edit"` or `"send"`.

- [ ] Write controller tests with fake audio and timers: PCM conversion and WAV header; X/Escape stops tracks; `finish("edit")` appends with spacing and does not submit; `finish("send")` sends combined draft once; empty result and failed transcription preserve draft; retry does not duplicate; profile change prevents send; disposal/permission failure cleans resources. Add Playwright tests for normal punctuation and IME, Enter/period/Escape only during recording, permission denial, no model tooltip accessible by keyboard/touch, elapsed timer, recording volume, mobile buttons and paper-plane/mic appearance.
- [ ] Run `cd internal/web/ui && bun test voice.test.js`; expect missing controller failures. Run focused Playwright specs and expect missing control failures.
- [ ] Implement browser capture with mono 16 kHz PCM WAV and five-minute cap; live volume meter and timer; cancellation/disposal; protect draft while recording; disable repeated actions during processing. Route completion through existing `sendMessage()`/request-id handling, never a parallel chat path. Keyboard shortcuts apply only during recording and must not swallow composition events. Implement the exact Preline idle controls and matching recording overlay from the spec, with `Type your message…`.
- [ ] Run `cd internal/web/ui && bun test voice.test.js`, `make web-build`, and `make web-browser-test`; expect PASS.

### Task 8: Embedded assets, documentation and release verification

**Files:** Modify `Makefile`, `internal/web/ui/package.json`, `internal/web/static/app.js`, `internal/web/static/app.css`, `internal/web/static/index.html`, `README.md`, and relevant platform/build docs.

**Interfaces:** Consumes all previous tasks; produces the built-in web UI and user-facing setup guidance.

- [ ] Add `voice.test.js` to the Bun test script and `make web-test`. Document manual model installation/deletion, shared disk use, profile-scoped choice, OpenRouter model discovery and costs, browser permission, offline versus provider privacy, keyboard shortcuts and known runtime/platform limitations.
- [ ] Run `make web-build` to rebuild embedded assets. Run `make web-test`, `make web-browser-test`, `make fmt lint test`, and `make build`; expect PASS. Run `make tidy` if dependencies changed, and check `git diff --check`.
- [ ] Manually test recording and edit/send/cancel in the supported browser; with explicit user permission for any model download or paid provider request and non-private audio fixtures, verify German and English against the local model and a configured OpenRouter STT model. Report unavailable permission, credentials or platforms and skipped real inference as unverified; never report mocks as real model validation.

## Review and handoff

Check the spec against each task, including disabled microphone explanation, no automatic download, shared-model deletion, accessibility, security and cleanup. Do not commit; ask the user before any git commit or delegated review.
