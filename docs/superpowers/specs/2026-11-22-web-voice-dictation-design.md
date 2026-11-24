# Web voice dictation design

## Purpose and scope

Add Paseo-inspired dictation to Noto's web composer. German is the primary language; English must also work. Speech is transcribed after the user finishes, not streamed as text. This is dictation only: no speech output, voice conversation mode, TUI microphone, or automatic model download.

## Composer behavior

- The idle composer matches the [Preline centered AI conversation example](https://preline.co/blocks/application-pages/ai-interface-pages/#centered-ai-conversation-page-with-carded-chat-bubbles): textarea above an inset bottom toolbar, with a muted 32px rounded microphone icon button immediately left of a 32px primary rounded send button displaying the same filled paper-plane icon. Use the generic placeholder “Type your message…”. Match the example's spacing, icon shapes, and hover/focus styling while retaining Noto's theme. Do not introduce unrelated example features such as attachments.
- The microphone starts recording after browser permission is granted. While recording, replace the composer controls with the approved layout: an X cancel icon at the far left, elapsed time, a live microphone-volume animation, a pencil icon for edit, and the same Preline-style paper-plane send button. Icon buttons have accessible names, tooltips and keyboard support.
- Enter or the send icon stops recording, transcribes, appends the transcript to the current draft, and sends the combined text using Noto's existing message submission path. Period or the pencil icon stops recording, transcribes, and appends the transcript to the draft for editing without sending. Escape or X cancels and discards the recording without changing the draft.
- These shortcuts apply only while recording; normal typing, punctuation, Enter behavior and input-method composition remain unchanged otherwise. The buttons provide the same actions on touch devices.
- Insert sensible spacing between an existing draft and transcript. An empty transcript is not sent and the draft remains intact. Show recording and processing states and meaningful errors; preserve the draft on errors. Stop microphone tracks on completion, cancellation, failure or page exit. Prevent duplicate finish/send actions. If the active profile changes during recording or transcription, do not send the result to a different profile; preserve the draft and present a clear error.
- When speech is not configured or its selected local model is not installed, disable the microphone and provide a tooltip explaining that Voice must be configured in Settings. Also make the explanation available to keyboard and touch users, not only via a disabled button's hover tooltip.

## Voice settings

Add a separate Voice page within the existing web Settings dialog. The user selects either local speech or provider speech. An unconfigured selection remains possible. The speech model is distinct from the chat model.

- **Local:** Offer multilingual Parakeet TDT 0.6B v3 int8, supporting German and English. The user explicitly starts its download; display approximate download size, status, progress and errors. Once installed, show a trash icon to delete it, with confirmation. A missing or partially downloaded model is not ready for recording. No model is fetched automatically. Show an actionable failure if the local runtime is unavailable.
- **Provider:** Reuse the active profile's configured provider endpoint and credentials. Offer a separate searchable speech model selector containing only models identified as supporting speech-to-text, not the ordinary chat model list. For OpenRouter, discover models with `GET /api/v1/models?output_modalities=transcription`; do not use audio-input support alone as a proxy for transcription endpoint support. Handle loading, empty results and discovery errors explicitly, and refresh available models when the active profile changes. If another provider cannot supply reliable speech-model capability metadata, show an explanatory unavailable state rather than mixing in unverified chat models. Explain that the provider must support the audio transcription endpoint, and show useful errors when it does not.
- Save speech selection and provider model per profile. Store the downloaded model in a shared Noto-managed model directory so profiles can reuse it; deleting the shared model makes local speech unavailable to all profiles until it is downloaded again. Identify this effect in the delete confirmation.

## Architecture

Paseo's web client captures microphone audio and sends PCM segments to its daemon; its local provider uses sherpa-onnx, and its OpenAI provider calls audio transcriptions. Noto can simplify this to a single bounded upload when recording finishes because live transcripts are not required.

1. Browser captures microphone audio and calculates volume for the animation. Normalize to a documented format supported by both backends (for example mono 16 kHz PCM WAV); do not assume every browser records the same MediaRecorder format. Enforce recording duration and upload limits and handle unsupported browsers or denied permission.
2. A same-origin Go API accepts the completed audio, validates size and format, and calls a speech-to-text interface using the active profile's chosen backend. Keep profile credentials on the server. Reuse the existing host/origin protections in `internal/web/server.go`.
3. The local adapter uses sherpa-onnx and the Parakeet v3 model on the Noto host, with CPU inference; select a Go integration and distribution strategy only after verifying native library, build and release compatibility for supported platforms. Isolate native dependencies so the normal Noto build and unsupported platforms degrade gracefully rather than break. Explicitly surface unsupported platforms in settings.
4. The provider adapter sends multipart audio and the configured speech model to the profile provider's OpenAI-compatible `/audio/transcriptions` endpoint, handling base URL conventions, cancellation, timeouts and provider errors. Never silently fall back to a remote provider from local speech.
5. Add settings persistence, model lifecycle and readiness APIs alongside Noto's existing profile settings and web service. Model downloads use verified expected artifacts, temporary files and atomic installation; deletion and inference must coordinate safely. Avoid persistent audio storage where feasible; if temporary files are required, restrict access and remove them on all paths. Do not log audio, transcripts or credentials.

## Failure handling and verification

Show a processing indicator while transcribing. Failed transcription must not send a message or clear the draft; expose retry or discard while the recorded audio remains available in memory. A chat send failure follows the existing send/retry semantics. Account for model-download failure, insufficient disk space, permission denial, silence, unsupported formats, provider failure and profile changes.

Test settings persistence and readiness, explicit download/deletion and partial-download cleanup, API limits and origin checks, local/provider routing with fakes, audio conversion, cancellation, empty results, failure/retry, and preservation of existing draft text. Browser tests cover the icon controls, shortcuts, disabled-state explanation and normal composer behavior. Verify actual German and English transcription with the local model on supported platforms; verify a provider adapter against a compatible test endpoint. Rebuild embedded web assets and run the repository's required Go and web checks.

## Implementation constraints and open engineering checks

- Confirm the precise Parakeet artifact, checksums, license, installed size and runtime/platform matrix before implementation; publish these accurately in settings and documentation.
- Choose and validate the sherpa-onnx Go binding and native-library packaging without compromising existing builds. If that cannot be done for a supported platform, report the limitation rather than promising local inference there.
- Actual provider model IDs and endpoint support depend on the active provider. A text model with the same name is not automatically a transcription model.

## References

- [Paseo voice architecture](https://paseo.sh/docs/voice.md)
- [Paseo browser audio capture](https://github.com/getpaseo/paseo/blob/main/packages/app/src/hooks/use-dictation-audio-source.web.ts)
- [Paseo dictation controls](https://github.com/getpaseo/paseo/blob/main/packages/app/src/components/dictation-controls.tsx)
- [Paseo local Parakeet provider](https://github.com/getpaseo/paseo/blob/main/packages/server/src/server/speech/providers/local/sherpa/sherpa-parakeet-stt.ts)
- [OpenRouter speech-to-text and model discovery](https://openrouter.ai/docs/guides/overview/multimodal/stt.md)
- [sherpa-onnx NeMo models](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/offline-transducer/nemo-transducer-models.html)
