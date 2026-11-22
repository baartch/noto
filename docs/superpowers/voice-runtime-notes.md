# Local voice runtime validation

Date: 2026-11-22

## Verified candidates and limits

- **Noto Go version:** `go.mod` and CI/release workflows use Go 1.27. `AGENTS.md` states a 1.26+ minimum; the implementation target is Go 1.27.
- **sherpa binding candidate:** `github.com/k2-fsa/sherpa-onnx-go/v2 v2.0.3`; its upstream `go.mod` pins platform modules at v2.0.3. This is a source-verified candidate pin, not yet imported or built in Noto. The official Go API requires cgo.
- **Upstream binding matrix (not Noto's supported release matrix):** Linux x86_64/aarch64/arm; macOS x86_64/aarch64; Windows x86_64/x86. The official Go API page says Windows examples may need the matching DLLs copied beside the example executable; this is an example-specific instruction, not a verified Noto packaging recipe.
- **Noto release constraint:** `.github/workflows/release.yml` builds Linux/amd64 natively on Ubuntu, but cross-compiles Windows/amd64 and Darwin/amd64 from Ubuntu. `GOOS=windows GOARCH=amd64 go env CGO_ENABLED` and the equivalent Darwin command both returned `0`; the Linux host returned `1`. Upstream's Linux/amd64 binding build excludes `musl` and embeds a module-cache rpath. Task 4 implements a native adapter only for `linux && amd64 && cgo && !musl`, stubs elsewhere, a `$ORIGIN/lib` rpath, and release packaging for the sherpa shared libraries. The Linux release tarball contains `noto` and `lib/`; keep them together when unpacking. The workflow checks `$ORIGIN/lib` and requires sherpa/ONNX dependencies to resolve from `dist/lib`, so the embedded module-cache RUNPATH cannot mask a missing bundle; it also runs the exact runtime test and executable in a clean environment. Go builds/tests and non-cgo cross-compiles pass; GitHub Actions release packaging itself has not run in this workspace.
- **Model:** official sherpa-onnx docs identify `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`, support German and English, and show extracted files totaling about **640 MiB** (rounded `ls -lh` output; exact installed bytes have not been measured). NVIDIA's model card specifies CC BY 4.0; Noto's Settings/docs must include attribution and a license link.
- **Current archive observation:** a HEAD request to the official `asr-models` URL returned 302 then 200 on 2026-11-22, final `Content-Length: 487211443` bytes (about 487.2 MB), `Last-Modified: Sat, 07 Nov 2026 05:51:02 GMT`, and opaque Azure ETag `"0x8DF3C931B6863C5"`. This is a point-in-time observation of a mutable release tag. Paseo's `packages/server/src/server/speech/providers/local/sherpa/model-catalog.ts:277-283` lists 487,170,055 bytes, 41,388 bytes less.
- **Pinned per-file integrity manifest:** the GitHub `.tar.bz2` archive itself has no published SHA-256 (`.sha256` returned 404), but the Sherpa-ONNX export is available in the maintainer's Hugging Face repository at immutable commit `ed5d745623472f97cfc2e6c2916ee248798fc5a8`. The model card says the sherpa-onnx project exported the same int8 Parakeet v3 files from NVIDIA's checkpoint. HF metadata provides LFS SHA-256 values for the three ONNX files; the user approved fetching only `tokens.txt` (93,939 bytes) to compute its SHA-256. Manifest:
  - `encoder.int8.onnx`: 652,282,294 bytes; SHA-256 `1afbe69c1ffdabee94755f42e4daf1d93549131a3da11356bda84fcef0f2c479`.
  - `decoder.int8.onnx`: 11,845,274 bytes; SHA-256 `0222a86147299d4dee90edf95b6d13827edb42ad8b7c9cf0b2582f030937c0b5`.
  - `joiner.int8.onnx`: 6,355,277 bytes; SHA-256 `cbd02f3696e9944970111fb439d9c3fd365a80374565f13f2b9c7007d253fc98`.
  - `tokens.txt`: 93,939 bytes; locally computed SHA-256 `d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d`.
  - Total required model files: 670,576,784 bytes (~639.6 MiB). GitHub's archive remains the smaller transport (~487.2 MB); Task 4 may use it only as a container and must verify every extracted required file against this pinned manifest before promotion. The archive itself is not cryptographically verified, and ONNX weights/archive were not downloaded during development. A changed archive must fail extraction/hash checks, never silently update the manifest.
- **Verification timing:** startup checks model file types/sizes only, then defers full SHA-256 verification until the first local transcription or an explicit download action. `ModelStatus.Verified` / `Verifying` expose that state so hashing ~640 MiB does not block web startup. Waiting for the single local inference slot is context-cancellable; a native inference already running cannot be interrupted by context cancellation.

## Download security requirements

- Use HTTPS only; validate every redirect against an explicit allowlist for the official GitHub release and its release-asset CDN. Do not forward credentials across redirects.
- Bound compressed and decompressed bytes. Reject absolute paths, `..`, links, special files and structural PAX overrides. Ignore only bounded regular documentation/test-WAV entries under the expected archive root; extract required regular model files to a private staging directory, verify each against the pinned per-file manifest, then atomically promote. Clean partial files on failure/cancellation.
- The official archive's layout was not inspected because no archive/model download was approved. The extractor requires the documented Parakeet directory root and the pinned filenames; if the transport archive differs, it fails closed. Fixture tests cover PAX metadata, benign bounded extras, traversal and repair of corrupt installs.
- Task 4 exercises installation with generated local tar fixtures and a fake recognizer, and a no-model native smoke test loads sherpa-onnx. The GitHub release workflow has not run in this workspace, and actual Parakeet inference remains unverified because no weights/archive were downloaded.

## Sources

- sherpa-onnx Go API, cgo requirement and upstream platform matrix: <https://k2-fsa.github.io/sherpa/onnx/go-api/index.html>
- sherpa-onnx v2.0.3 module manifest: <https://raw.githubusercontent.com/k2-fsa/sherpa-onnx-go/v2.0.3/go.mod>
- Linux native build constraints and module-cache rpath: <https://raw.githubusercontent.com/k2-fsa/sherpa-onnx-go-linux/master/build_linux_amd64.go>
- Noto Go target: `go.mod`; cross-build/package targets: `.github/workflows/release.yml`.
- Parakeet v3 artifact, language support and rounded extracted-size example: <https://k2-fsa.github.io/sherpa/onnx/pretrained_models/offline-transducer/nemo-transducer-models.html>
- Sherpa-ONNX Hugging Face export at pinned commit: <https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/tree/ed5d745623472f97cfc2e6c2916ee248798fc5a8>
- Pinned HF file-integrity metadata API: <https://huggingface.co/api/models/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/tree/ed5d745623472f97cfc2e6c2916ee248798fc5a8?recursive=true&expand=true>
- NVIDIA Parakeet v3 model card and license: <https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3>
- Paseo's model size/catalog entry: <https://github.com/getpaseo/paseo/blob/main/packages/server/src/server/speech/providers/local/sherpa/model-catalog.ts#L277-L283>
- Artifact URL: <https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2>

## Verification commands and results

- `go env CGO_ENABLED GOOS GOARCH` — `1`, `linux`, `amd64`.
- `GOOS=windows GOARCH=amd64 go env CGO_ENABLED` — `0`.
- `GOOS=darwin GOARCH=amd64 go env CGO_ENABLED` — `0`.
- `curl -sSI -L https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2 | grep -iE '^(HTTP/|content-length:|etag:|last-modified:)'` — 302 then 200; headers recorded above.
- `curl -sSI https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2.sha256 -o /dev/null -w '%{http_code}\n'` — `404`.
- `curl -L --fail --silent --show-error https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/ed5d745623472f97cfc2e6c2916ee248798fc5a8/tokens.txt -o /tmp/opencode/noto-parakeet-v3-tokens.txt` — user-approved; 93,939 bytes.
- `sha256sum /tmp/opencode/noto-parakeet-v3-tokens.txt` — `d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d`.
- `make build` — PASS with the repository's existing dependencies only; this does **not** test the sherpa binding.
- `go test ./internal/store ./internal/web` — PASS.
- Local model inference and binding/linker/package validation — **not run**; no ONNX weights or archive were downloaded, and Noto's proposed native integration has not been implemented. Only the user-approved `tokens.txt` file was downloaded to compute its digest.
