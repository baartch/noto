# Web Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task by task. Checkboxes track progress.

**Goal:** Add a web settings page for selecting an existing profile and its LLM model, with explicit Save and Cancel actions.

**Architecture:** Keep one server-wide active profile. Extend the web service with a synchronized, replaceable profile runtime and a settings backend implemented by the app layer. Fetch draft choices without changing active state; a single validated save persists the selection, updates the runtime, and causes all tabs to refresh their profile-scoped data.

**Tech stack:** Go 1.26+, SQLite, existing profile and provider services, vanilla JavaScript, Bun, Tailwind CSS, Lucide.

**Spec:** The approved design in the conversation: a top-right gear opens a separate settings view; profile changes load that profile's saved model and model list; Save applies both selections and returns to chat; Cancel discards both; save/list errors remain visible; saving is blocked during a message; all tabs share the active selection.

## Global constraints

- Do not add profile creation, provider configuration, or unrelated settings.
- Models and provider credentials are profile-specific. Credentials and raw upstream errors never leave the server.
- Profile selection persists via the existing active-profile marker; model selection persists in that profile's active provider configuration.
- Treat a profile without a configured provider as selectable with an empty model; return to the existing read-only setup state.
- Failed saves must leave the previous runtime active. Restore previously persisted values on a recoverable save error; if restoration itself fails, stop further mutations and surface a safe recovery error rather than falsely reporting success.
- Serialize switching, sends, reads, shutdown and resource retirement. Do not close a profile database while background note extraction or a request still uses it.
- No new dependencies are expected. If dependencies change, run `make tidy`.
- Rebuild `internal/web/static/` from `internal/web/ui/`; run `make fmt lint test` after implementation.
- Make one atomic commit per completed and reviewed task; do not create separate commit-plan files.

## Review focus

1. A profile without provider credentials remains selectable without exposing secrets or enabling chat (Tasks 1–3).
2. A send or background memory extraction overlaps Save; no session or database is closed prematurely (Task 2).
3. A failed persistence step or session startup does not activate a partial selection (Task 2).
4. Rapid profile selection and delayed network responses must not show models or data from the wrong profile (Tasks 1, 3, 4).
5. A stale browser tab must not send its draft into a different profile after another tab saves (Tasks 2 and 4).

## File map

- `internal/web/settings.go` and `internal/web/dto.go`: settings interfaces, safe DTOs and error types.
- `internal/web/server.go`: settings routes, validation and safe error mapping.
- `internal/web/service.go`: runtime synchronization, active revision, send and shutdown coordination.
- `internal/app/web_settings.go` and `internal/app/web_cmd.go`: profile discovery, provider model lookup, runtime creation, persistence and cleanup.
- `internal/store/provider_config_repo.go` and `internal/config/active_profile.go`: narrowly scoped persistence helpers if needed for transaction/rollback and atomic marker replacement.
- `internal/web/ui/index.html`, `internal/web/ui/app.js`, `internal/web/ui/state.js`, `internal/web/ui/input.css`: settings view, draft state, navigation and shared-profile refresh.
- `internal/web/static/`: generated embedded assets.
- `internal/web/server_test.go`, `internal/web/service_test.go`, `internal/app/web_cmd_test.go`, new `internal/app/web_settings_test.go`, `internal/web/ui/app.test.js`, and `internal/web/ui/state.test.js`: targeted coverage.

## Task 1: Read-only settings API

**Files:** `internal/web/settings.go`, `internal/web/dto.go`, `internal/web/server.go`, `internal/app/web_settings.go`, `internal/app/web_cmd.go`, `internal/web/server_test.go`, `internal/app/web_settings_test.go`.

**Interfaces:** Define `SettingsBackend` with `Profiles(context.Context) ([]SettingsProfileDTO, error)` and `Models(context.Context, string) (ProfileModelsDTO, error)`. `SettingsProfileDTO` contains `slug` and `name`; `ProfileModelsDTO` contains `profile_slug`, `saved_model`, `models` (model IDs), and `setup_required`. Use an exact slug from discovered profiles, not a user-supplied filesystem path.

- [x] Add `TestSettingsReadEndpoints` covering `GET /api/settings` returning active slug and discovered profiles, and `GET /api/settings/models?profile=<slug>` returning that profile's saved model and its provider's model IDs. Verify discovery/model lookup never changes the active marker or stored model.
- [x] Add tests for unknown slugs (404), missing provider (empty models, `setup_required: true`), upstream failure (sanitized 502), and credentials absent from all response bodies. Include wrong method, invalid host and foreign origin checks.
- [x] Run `go test ./internal/web ./internal/app -run 'TestSettings'`; confirm the new tests fail because the routes do not exist.
- [x] Implement the DTOs and backend. Reuse profile discovery, `openProfileDB`, and `provider.ListModels`; respect `loadProviderConfig` for environment overrides and preserve stored model data when credentials are absent or invalid. Close every temporary DB. Return the saved model even if it is absent from a successful provider list, so an existing selection is not silently lost. Never forward provider response bodies or keys.
- [x] Wire the backend into `runWeb` and register the GET routes using existing host/origin, cache and method safeguards.
- [x] Repeat the targeted Go tests and confirm they pass.

## Task 2: Synchronized, persistent Save

**Files:** `internal/web/settings.go`, `internal/web/dto.go`, `internal/web/server.go`, `internal/web/service.go`, `internal/app/web_settings.go`, `internal/app/web_cmd.go`, `internal/chat/session.go`, `internal/chat/session_test.go`, `internal/config/active_profile.go`, `internal/config/active_profile_test.go`, `internal/web/settings_test.go`, `internal/web/server_test.go`, `internal/app/web_settings_test.go`, `internal/app/web_cmd_test.go`, `internal/web/ui/app.js`, `internal/web/ui/app.test.js`, `internal/web/ui/state.js`, `internal/web/ui/state.test.js`, generated `internal/web/static/app.js`.

**Interfaces:** Add `POST /api/settings` with `{ "profile_slug": string, "model": string, "revision": integer }`; return the updated state including the new revision. Add the active profile slug and monotonically increasing process-local `revision` to `GET /api/state`. A stale revision returns 409 without changes. Extend message requests with the profile slug observed by the tab; reject a mismatch with 409 before starting or retrying a send.

- [x] Add `TestSaveSettingsSwitchesProfileAndModel`: with two isolated profiles and fake model providers, save profile B/model B2; assert state, message history and memory come from B, the stored model is B2, and the active marker is B. Restart the service and assert B/B2 remain selected. Also test changing only the model of the current profile without creating a second session.
- [x] Add tests for unknown profiles, unknown model IDs, stale revision, a profile without setup, JSON validation, same-origin enforcement, and a stale-profile send. For a configured provider accept the saved model or an ID returned by that profile's current provider; reject all other IDs.
- [x] Add deterministic failure-injection tests for opening the destination DB, constructing the destination session, model persistence, active-marker persistence, and DB commit. Each ordinary failure keeps the old runtime, marker and stored model; rollback failure yields a safe recovery error and rejects further writes. Verify no credential-bearing error is returned.
- [x] Add concurrency tests: block a send while attempting Save (409, no changes); block a message/history read while switching (DB remains open until the read exits); block memory extraction while switching (Save waits or returns a retryable conflict; never close its DB early); concurrent saves with the same revision cannot both succeed; shutdown closes each live session and DB exactly once. Run these with `go test -race ./internal/web ./internal/app`.
- [x] Run `go test ./internal/web ./internal/app -run 'TestSaveSettings|TestSettings'`; confirm new tests fail before implementation.
- [x] Replace the fixed-lifetime profile binding with an owned runtime containing profile, DB, repositories, session, model and setup state. Protect runtime snapshots and their entire use with synchronization; reserve a switch before slow preparation so new sends cannot race it. Keep request-ID deduplication scoped to the original profile.
- [x] Prepare a destination runtime only when changing profiles. `chat.NewSession` archives active conversations and creates one, so do not create a second session for the currently active profile. On preparation failure close only the candidate and retain the original. Before retiring the old runtime, drain in-flight reads and pending extraction; `Session.Close` waits at most four seconds, so do not assume it guarantees extraction has finished.
- [x] Persist the selected model through a destination-DB transaction and the profile marker via an atomic file replacement. Record prior values and compensate on marker/commit failure before exposing the new runtime; if compensation fails, enter a recovery-required state. Do not claim crash-atomicity across SQLite and a separate marker file: document and test ordinary failure recovery, and keep the runtime unpublished until persistence succeeds.
- [x] For same-profile changes, persist first, then update `Session.SetModel` and reuse `applySessionModelMetadata`, including extractor fallback behavior. For a different profile, build the session with the selected model, then publish it only after successful persistence; clean up the old runtime after its users drain. Return updated state and increment revision only after publication.
- [x] Include the active `profile_slug` in existing chat sends and rebuild embedded assets so this API-contract commit leaves chat working; Task 3 adds the settings view and draft controls.
- [x] Repeat targeted tests and `go test -race ./internal/web ./internal/app`; confirm all pass.

## Task 3: Settings view and draft behavior

**Files:** `internal/web/ui/index.html`, `internal/web/ui/app.js`, `internal/web/ui/state.js`, `internal/web/ui/input.css`, `internal/web/ui/app.test.js`, `internal/web/ui/state.test.js`.

**Interfaces:** The view reads `GET /api/settings` and `GET /api/settings/models?profile=<slug>`; Save sends the draft and opening revision to `POST /api/settings`; Cancel only discards local draft. Use accessible labels `Settings`, `Profile`, `LLM model`, `Save`, and `Cancel`.

- [x] Add UI tests for the visible top-right gear, switching to the settings view, loading the active profile's model, and choosing profile B without making a POST. Verify a delayed profile A models response cannot overwrite a newer profile B selection.
- [x] Add tests for Cancel (no POST, return to the unchanged chat), Save (one POST, then return to refreshed chat), provider/model-list and save errors (stay on settings, show safe message), missing provider (empty disabled model selector with setup guidance), and saving while sending (disabled UI and server conflict handling).
- [x] Run `make web-test`; confirm the new tests fail before the view exists.
- [x] Add a responsive settings view and gear, using existing colors and Lucide icons. Hide chat/navigation while settings is open; return focus to the gear when leaving. Keep the gear accessible on mobile, where the model label is currently hidden.
- [x] Implement draft state, async profile-model lookup with abort or request identity checks, error/loading states, and Save/Cancel handlers. If fetching models fails, do not submit stale choices; retain the page and allow retry. For an unconfigured profile submit an empty model.
- [x] On successful Save, clear stale profile-specific messages, memory, cursors and optimistic state before loading the returned profile; do not automatically resend a pending draft. Leave the page open on failure.
- [x] Run `make web-test`; confirm all UI tests pass.

## Task 4: Cross-tab consistency and verification

**Files:** `internal/web/ui/app.js`, `internal/web/ui/state.js`, `internal/web/ui/app.test.js`, `internal/web/ui/state.test.js`, `internal/web/server_test.go`, `internal/web/static/*`.

- [ ] Add tests simulating tab A switching profiles while tab B is idle, drafting, viewing settings (including a stale-revision reload), loading older messages, or receiving a delayed memory response. Assert B detects the changed revision, clears old profile data and stale chat status, reloads state/history/memory, preserves unsent text without sending it, and cannot overwrite the newer selection from an outdated settings draft.
- [ ] Run `make web-test`; confirm the cross-tab tests fail before implementation.
- [ ] Refresh `/api/state` on a bounded timer while the page is visible and on focus/visibility return. On a revision change abort or invalidate outstanding profile-scoped requests and polling, reset pagination and pending display, and reload the new profile. Use the message profile guard from Task 2 so a stale tab cannot send to the wrong profile between polls. On a settings revision conflict, retain the view, show an explanatory error and reload choices before another save.
- [ ] Rebuild embedded assets with `make web-build`; run `make web-test` and `go test -race ./internal/web ./internal/app` and confirm they pass.
- [ ] Run `make fmt lint test`; report any missing tool or failure rather than claiming success. Manually verify keyboard navigation, mobile layout, Save/Cancel, profile-specific history, missing provider and two-tab behavior against the running web server.

## Known boundary

SQLite and the active-profile marker are separate persistence resources. The plan covers rollback for reported write failures, not a process crash or power loss between their commits. If crash-atomic cross-resource selection becomes a requirement, design a durable recovery journal separately rather than assuming a transaction spans both resources.
