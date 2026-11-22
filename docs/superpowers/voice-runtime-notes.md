# Local voice runtime validation

Date: 2026-11-22

## Verified candidates and limits

- **Noto Go version:** `go.mod` and CI/release workflows use Go 1.27. `AGENTS.md` states a 1.26+ minimum; the implementation target is Go 1.27.
- **sherpa binding candidate:** `github.com/k2-fsa/sherpa-onnx-go/v2 v2.0.3`; its upstream `go.mod` pins platform modules at v2.0.3. This is a source-verified candidate pin, not yet imported or built in Noto. The official Go API requires cgo.
- **Upstream binding matrix (not Noto's supported release matrix):** Linux x86_64/aarch64/arm; macOS x86_64/aarch64; Windows x86_64/x86. The official Go API page says Windows examples may need the matching DLLs copied beside the example executable; this is an example-specific instruction, not a verified Noto packaging recipe.
- **Noto release constraint:** `.github/workflows/release.yml` builds Linux/amd64 natively on Ubuntu, but cross-compiles Windows/amd64 and Darwin/amd64 from Ubuntu. `GOOS=windows GOARCH=amd64 go env CGO_ENABLED` and the equivalent Darwin command both returned `0`; the Linux host returned `1`. The current release job packages only the executable. The sherpa Linux module contains shared `.so` libraries and its cgo build file embeds an rpath into the Go module cache. Therefore local inference is not release-ready. Proposed approach to validate in Task 4: compile the native adapter only for `linux && amd64 && cgo`, use an explicit unsupported stub for all other targets and `!cgo`, and package the required Linux shared libraries with a tested `$ORIGIN/lib` runtime search path. If that cannot be made reliable, report local speech unavailable in release builds. This proposal is not yet integration-tested.
- **Model:** official sherpa-onnx docs identify `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`, support German and English, and show extracted files totaling about **640 MiB** (rounded `ls -lh` output; exact installed bytes have not been measured). NVIDIA's model card specifies CC BY 4.0; Noto's Settings/docs must include attribution and a license link.
- **Current archive observation:** a HEAD request to the official `asr-models` URL returned 302 then 200 on 2026-11-22, final `Content-Length: 487211443` bytes (about 487.2 MB), `Last-Modified: Sat, 07 Nov 2026 05:51:02 GMT`, and opaque Azure ETag `"0x8DF3C931B6863C5"`. This is a point-in-time observation of a mutable release tag. Paseo's `packages/server/src/server/speech/providers/local/sherpa/model-catalog.ts:277-283` lists 487,170,055 bytes, 41,388 bytes less.
- **Digest:** upstream did not expose a cryptographic digest in the inspected metadata; the sibling `.sha256` URL returned 404. Paseo's `model-catalog.ts:277-283` records the expected size but no hash; its `model-downloader.ts` checks archive size/required files but does not hash the archive. Do not invent a SHA-256. The archive, exact-size metadata and extracted layout remain unverified locally because the user declined the download on 2026-11-22. The plan's digest/inference gate is therefore unresolved. Do not ship an installer claiming cryptographic verification until a trusted digest is available. If the upstream asset is ever re-uploaded, fail closed and re-audit updated metadata rather than silently accepting a changed size/ETag.

## Download security requirements

- Use HTTPS only; validate every redirect against an explicit allowlist for the official GitHub release and its release-asset CDN. Do not forward credentials across redirects.
- Bound compressed and decompressed bytes. Reject absolute paths, `..`, links and unexpected archive entries; extract only required regular files to a private staging directory, verify layout, then atomically promote. Clean all partial files on failure/cancellation.
- These controls and package/runtime linking still require tests in Task 4; this note is not evidence that an installer exists.

## Sources

- sherpa-onnx Go API, cgo requirement and upstream platform matrix: <https://k2-fsa.github.io/sherpa/onnx/go-api/index.html>
- sherpa-onnx v2.0.3 module manifest: <https://raw.githubusercontent.com/k2-fsa/sherpa-onnx-go/v2.0.3/go.mod>
- Linux native build constraints and module-cache rpath: <https://raw.githubusercontent.com/k2-fsa/sherpa-onnx-go-linux/master/build_linux_amd64.go>
- Parakeet v3 artifact, language support and rounded extracted-size example: <https://k2-fsa.github.io/sherpa/onnx/pretrained_models/offline-transducer/nemo-transducer-models.html>
- NVIDIA Parakeet v3 model card and license: <https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3>
- Paseo's model size/catalog entry: <https://github.com/getpaseo/paseo/blob/main/packages/server/src/server/speech/providers/local/sherpa/model-catalog.ts#L277-L283>
- Artifact URL: <https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2>

## Verification commands and results

- `go env CGO_ENABLED GOOS GOARCH` — `1`, `linux`, `amd64`.
- `GOOS=windows GOARCH=amd64 go env CGO_ENABLED` — `0`.
- `GOOS=darwin GOARCH=amd64 go env CGO_ENABLED` — `0`.
- `curl -sSI -L https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2 | grep -iE '^(HTTP/|content-length:|etag:|last-modified:)'` — 302 then 200; headers recorded above.
- `curl -sSI https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2.sha256 -o /dev/null -w '%{http_code}\n'` — `404`.
- `make build` — PASS with the repository's existing dependencies only; this does **not** test the sherpa binding.
- `go test ./internal/store ./internal/web` — PASS.
- Local model inference and binding/linker/package validation — **not run**; no model was downloaded and Noto's proposed native integration has not been implemented.
