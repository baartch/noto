# Web voice dictation

Noto's web composer can transcribe a recording and either append the text to the draft or
send it through the normal chat path. Transcription starts only after recording stops;
there is no live transcript or automatic backend fallback.

## Configure a profile

1. Start `noto web` and open **Settings → Voice**.
2. Choose **Local Parakeet** or **Profile provider**. Voice settings are saved separately
   for each profile; the selected speech model is separate from the chat model.
3. The microphone stays disabled until that profile's voice choice is ready.

### Local Parakeet

Local dictation uses multilingual Parakeet TDT 0.6B v3 int8. It is intended for German and
English. The runtime currently works only in a Linux amd64 glibc build with cgo enabled;
musl builds, Windows, macOS, other architectures, and cgo-disabled builds do not include
the sherpa-onnx runtime. The local recognizer runs on CPU.

The compressed model download is about 487 MB and its installed files use about 640 MiB.
Nothing downloads automatically: choose **Download model** in Settings and wait for its
verification/installation to finish. Noto checks the expected files before enabling local
dictation. Use **Delete model** in Settings to remove it. The installation is shared by all
profiles, so deleting it disables local dictation for every profile until it is downloaded
again. Prefer these controls over manually unpacking or removing files; Noto coordinates
verification, installation, inference, and deletion.

### Provider speech-to-text

Provider transcription uses the active profile's configured OpenAI-compatible endpoint
and credentials. Noto currently offers provider models only when the endpoint is OpenRouter
and its model metadata identifies transcription output. Discovery uses
`GET /api/v1/models?output_modalities=transcription`; audio-input capability alone is not
treated as proof that a model supports transcription. For other endpoints, model discovery
is reported unavailable rather than showing unverified chat models. The chosen speech model
must also support the provider's `/audio/transcriptions` endpoint.

OpenRouter model availability and pricing can change. Check the current model pricing and
audio billing details shown by OpenRouter before selecting a model; charges, minimums, and
retention are governed by the provider. Noto does not set or guarantee a transcription
price. Model discovery is a provider metadata request; each transcription sends one
recording to the selected profile provider and may incur charges. The chat model remains
independent.

## Microphone, privacy, and controls

The browser asks for microphone permission when you start a recording. Grant permission to
the local Noto page (`http://127.0.0.1:<port>`); browser and operating-system privacy
settings can revoke it later. Noto records mono 16 kHz PCM WAV, for at most five minutes,
and uploads it to the same-origin local Noto server after you finish. Noto does not create
an audio archive or save recordings in profile history. With local Parakeet, audio is
transcribed on the Noto host and is not sent to the configured chat provider. With provider
speech, the WAV is sent from the Noto server to the active profile's configured provider;
the provider's processing, logging, and retention policies then apply. If you choose Send,
the resulting text is saved as an ordinary chat message; choosing Edit only appends it to
the current draft.

While recording:

- **Enter** or the send button stops, transcribes, and sends the combined draft.
- **Period (`.`)** or the pencil button stops, transcribes, and appends to the draft for
  editing without sending.
- **Escape** or the X button cancels and discards the recording without changing the draft.

These keyboard shortcuts apply only during recording. Focused controls keep their normal
keyboard behavior; normal punctuation, Enter submission, and IME composition remain intact
otherwise. Touch users can use the visible controls. Silence, denied permission, unsupported
browser audio APIs, or a provider error leaves the existing draft intact and shows an error.

The browser needs `getUserMedia` and Web Audio support, and microphone access must be
allowed for the page. Automated browser coverage currently exercises desktop and mobile
Chrome with mocked audio and transcription endpoints; it does not validate real microphone
hardware, real model accuracy, or provider billing. Actual German and English inference
must be validated separately on a supported Linux host or against a configured provider.
