package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"noto/internal/store"
	"noto/internal/voice"
)

const maxVoiceWAVBytes = 9_600_044

const (
	// VoiceProviderLocal selects the shared on-device Parakeet model.
	VoiceProviderLocal = "local"
	// VoiceProviderProvider selects the active profile's configured speech provider.
	VoiceProviderProvider = "provider"
)

var (
	// ErrVoiceBackendUnavailable indicates voice services were not configured at startup.
	ErrVoiceBackendUnavailable = errors.New("web: voice backend unavailable")
	// ErrVoiceOperationConflict indicates a download or delete is already in progress.
	ErrVoiceOperationConflict = errors.New("web: voice model operation conflict")
	// ErrVoiceNotReady indicates the saved backend/model selection is incomplete or unsupported.
	ErrVoiceNotReady = errors.New("web: voice selection is not ready")
	// ErrInvalidVoiceSettings indicates a malformed backend/model combination.
	ErrInvalidVoiceSettings = errors.New("web: invalid voice settings")
	// ErrVoiceModelUnverified indicates a provider model is not transcription-capable.
	ErrVoiceModelUnverified = errors.New("web: voice model is not verified for transcription")
)

// VoiceBackend owns profile-specific speech settings and transcription routing.
type VoiceBackend interface {
	Get(context.Context, string) (VoiceDTO, error)
	Save(context.Context, string, store.VoiceSettings, uint64) error
	Models(context.Context, string) ([]string, error)
	Transcribe(context.Context, string, []byte) (string, error)
	Download(context.Context) error
	Delete(context.Context) error
	ModelStatus() voice.ModelStatus
}

// VoiceDTO contains the selected backend/model and browser-safe readiness state.
// It deliberately has no field for provider credentials.
type VoiceDTO struct {
	ProfileSlug string            `json:"profile_slug"`
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	Ready       bool              `json:"ready"`
	ModelStatus voice.ModelStatus `json:"model_status"`
	Reason      string            `json:"reason,omitempty"`
	Revision    uint64            `json:"revision"`
}

// VoiceModelsDTO contains transcription-capable model IDs for one profile.
type VoiceModelsDTO struct {
	ProfileSlug string   `json:"profile_slug"`
	Models      []string `json:"models"`
	Revision    uint64   `json:"revision"`
}

// VoiceModelStatusDTO exposes lifecycle state without raw filesystem/runtime errors.
type VoiceModelStatusDTO struct {
	ModelStatus voice.ModelStatus `json:"model_status"`
	Reason      string            `json:"reason,omitempty"`
}

// NewHandlerWithVoice binds the optional voice API alongside existing handlers.
func NewHandlerWithVoice(service *Service, origin string, settingsBackend SettingsBackend, voiceBackend VoiceBackend) http.Handler {
	expectedHost := ""
	if parsed, err := url.Parse(origin); err == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.User == nil {
		expectedHost = parsed.Host
	}
	return newHandler(service, origin, expectedHost, settingsBackend, voiceBackend)
}

// SaveVoiceSettings serializes profile-scoped voice settings against runtime
// replacement and rejects writes based on a stale web revision.
func (s *Service) SaveVoiceSettings(ctx context.Context, backend VoiceBackend, profileSlug string, settings store.VoiceSettings, revision uint64) error {
	if backend == nil || strings.TrimSpace(profileSlug) == "" {
		return ErrVoiceBackendUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return ErrServiceStopped
	}
	if s.recoveryRequired {
		s.mu.Unlock()
		return ErrSettingsRecoveryRequired
	}
	if revision != s.revision {
		s.mu.Unlock()
		return ErrSettingsRevisionConflict
	}
	if s.sending || s.switching {
		s.mu.Unlock()
		return ErrSettingsBusy
	}
	s.switching = true
	s.mu.Unlock()

	s.runtimeMu.Lock()
	if s.runtime == nil {
		s.runtimeMu.Unlock()
		s.clearSwitching()
		return ErrServiceStopped
	}
	if s.runtime.Profile == nil || s.runtime.Profile.Slug != profileSlug {
		s.runtimeMu.Unlock()
		s.clearSwitching()
		return ErrProfileMismatch
	}
	if err := ctx.Err(); err != nil {
		s.runtimeMu.Unlock()
		s.clearSwitching()
		return err
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		s.runtimeMu.Unlock()
		s.clearSwitching()
		return ErrServiceStopped
	}
	err := backend.Save(ctx, profileSlug, settings, revision)
	s.runtimeMu.Unlock()

	s.mu.Lock()
	if err == nil {
		s.revision++
	}
	s.switching = false
	s.mu.Unlock()
	return err
}

// TranscribeVoice pins the active profile runtime for the complete dispatch so
// a profile switch cannot redirect old audio to another profile's settings.
func (s *Service) TranscribeVoice(ctx context.Context, backend VoiceBackend, profileSlug string, audio []byte) (string, error) {
	if backend == nil {
		return "", ErrVoiceBackendUnavailable
	}
	if strings.TrimSpace(profileSlug) == "" {
		return "", ErrProfileMismatch
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	s.mu.Lock()
	stopped, switching := s.stopped, s.switching
	s.mu.Unlock()
	if stopped || s.runtime == nil || s.runtime.Profile == nil {
		return "", ErrServiceStopped
	}
	if switching {
		return "", ErrSettingsBusy
	}
	if s.runtime.Profile.Slug != profileSlug {
		return "", ErrProfileMismatch
	}
	return backend.Transcribe(ctx, profileSlug, audio)
}

func handleVoiceRoute(w http.ResponseWriter, r *http.Request, service *Service, backend VoiceBackend) bool {
	switch r.URL.Path {
	case "/api/voice":
		if backend == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
			return true
		}
		switch r.Method {
		case http.MethodGet:
			profileSlug, ok := requiredVoiceProfile(w, r)
			if !ok {
				return true
			}
			revision := serviceRevision(service)
			dto, err := backend.Get(r.Context(), profileSlug)
			if err != nil {
				writeVoiceError(w, err, "voice settings could not be loaded")
				return true
			}
			dto.ProfileSlug = profileSlug
			dto.Revision = revision
			dto = sanitizeVoiceDTO(dto)
			writeJSON(w, http.StatusOK, dto)
		case http.MethodPost:
			handleVoiceSave(w, r, service, backend)
		default:
			writeMethodError(w, "GET, POST")
		}
		return true

	case "/api/voice/models":
		if r.Method != http.MethodGet {
			writeMethodError(w, "GET")
			return true
		}
		if backend == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
			return true
		}
		profileSlug, ok := requiredVoiceProfile(w, r)
		if !ok {
			return true
		}
		revision := serviceRevision(service)
		models, err := backend.Models(r.Context(), profileSlug)
		if err != nil {
			writeVoiceError(w, err, "speech models are unavailable")
			return true
		}
		if models == nil {
			models = []string{}
		}
		result := VoiceModelsDTO{ProfileSlug: profileSlug, Models: models}
		result.Revision = revision
		writeJSON(w, http.StatusOK, result)
		return true

	case "/api/voice/transcriptions":
		if r.Method != http.MethodPost {
			writeMethodError(w, "POST")
			return true
		}
		handleVoiceTranscription(w, r, service, backend)
		return true

	case "/api/voice/local-model":
		if backend == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
			return true
		}
		switch r.Method {
		case http.MethodGet:
			writeVoiceModelStatus(w, backend.ModelStatus(), http.StatusOK)
		case http.MethodDelete:
			status := backend.ModelStatus()
			if status.Downloading || status.Deleting {
				writeAPIError(w, http.StatusConflict, "model_operation_conflict", "the local speech model is busy")
				return true
			}
			if err := backend.Delete(r.Context()); err != nil {
				writeVoiceError(w, err, "the local speech model could not be deleted")
				return true
			}
			writeVoiceModelStatus(w, backend.ModelStatus(), http.StatusOK)
		default:
			writeMethodError(w, "GET, DELETE")
		}
		return true

	case "/api/voice/local-model/download":
		if r.Method != http.MethodPost {
			writeMethodError(w, "POST")
			return true
		}
		if backend == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
			return true
		}
		status := backend.ModelStatus()
		if status.Downloading || status.Deleting {
			writeAPIError(w, http.StatusConflict, "model_operation_conflict", "the local speech model is busy")
			return true
		}
		if err := backend.Download(r.Context()); err != nil {
			writeVoiceError(w, err, "the local speech model download could not be started")
			return true
		}
		writeVoiceModelStatus(w, backend.ModelStatus(), http.StatusAccepted)
		return true
	}
	return false
}

func handleVoiceSave(w http.ResponseWriter, r *http.Request, service *Service, backend VoiceBackend) {
	if service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "json_required", "request content type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		ProfileSlug string `json:"profile_slug"`
		Provider    string `json:"provider"`
		Model       string `json:"model"`
		Revision    uint64 `json:"revision"`
	}
	if err := decoder.Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "voice settings request exceeds 64 KiB")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body is invalid")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return
	}
	if strings.TrimSpace(request.ProfileSlug) == "" || request.Revision == 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_voice_settings", "profile_slug and nonzero revision are required")
		return
	}
	switch request.Provider {
	case "", VoiceProviderLocal, VoiceProviderProvider:
	default:
		writeAPIError(w, http.StatusBadRequest, "invalid_voice_settings", "voice provider is invalid")
		return
	}
	settings := store.VoiceSettings{Provider: request.Provider, Model: request.Model}
	if err := service.SaveVoiceSettings(r.Context(), backend, request.ProfileSlug, settings, request.Revision); err != nil {
		writeVoiceError(w, err, "voice settings could not be saved")
		return
	}
	revision := serviceRevision(service)
	dto, err := backend.Get(r.Context(), request.ProfileSlug)
	if err != nil {
		writeVoiceError(w, err, "voice settings could not be loaded")
		return
	}
	dto.ProfileSlug = request.ProfileSlug
	dto.Revision = revision
	dto = sanitizeVoiceDTO(dto)
	writeJSON(w, http.StatusOK, dto)
}

func handleVoiceTranscription(w http.ResponseWriter, r *http.Request, service *Service, backend VoiceBackend) {
	if backend == nil || service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
		return
	}
	profileSlug, ok := requiredVoiceProfile(w, r)
	if !ok {
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "audio/wav" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "wav_required", "request content type must be audio/wav")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxVoiceWAVBytes)
	audio, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "audio_too_large", "audio exceeds the five-minute WAV limit")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_audio", "audio could not be read")
		return
	}
	if err := voice.ValidateWAV(audio); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_audio", "audio must be mono 16 kHz PCM16 WAV no longer than five minutes")
		return
	}
	text, err := service.TranscribeVoice(r.Context(), backend, profileSlug, audio)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		writeVoiceError(w, err, "transcription failed")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Text string `json:"text"`
	}{Text: text})
}

func requiredVoiceProfile(w http.ResponseWriter, r *http.Request) (string, bool) {
	profileSlug := strings.TrimSpace(r.URL.Query().Get("profile_slug"))
	if profileSlug == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_profile", "profile_slug is required")
		return "", false
	}
	return profileSlug, true
}

func serviceRevision(service *Service) uint64 {
	if service == nil {
		return 0
	}
	return service.State().Revision
}

func writeVoiceModelStatus(w http.ResponseWriter, status voice.ModelStatus, code int) {
	status, reason := sanitizeVoiceStatus(status, "")
	if reason == "" && !status.Available {
		reason = "The local speech runtime is unavailable on this platform."
	}
	writeJSON(w, code, VoiceModelStatusDTO{ModelStatus: status, Reason: reason})
}

func sanitizeVoiceStatus(status voice.ModelStatus, reason string) (voice.ModelStatus, string) {
	if status.Error != "" {
		status.Error = ""
		if reason == "" {
			reason = "The local speech model operation failed."
		}
	}
	return status, reason
}

func sanitizeVoiceDTO(dto VoiceDTO) VoiceDTO {
	statusReason := ""
	if dto.Provider == VoiceProviderLocal {
		statusReason = dto.Reason
	}
	dto.ModelStatus, statusReason = sanitizeVoiceStatus(dto.ModelStatus, statusReason)
	if dto.Provider == VoiceProviderLocal && dto.Reason == "" {
		dto.Reason = statusReason
	}
	return dto
}

func writeVoiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrVoiceBackendUnavailable):
		writeAPIError(w, http.StatusServiceUnavailable, "voice_unavailable", "voice services are unavailable")
	case errors.Is(err, ErrSettingsProfileNotFound):
		writeAPIError(w, http.StatusNotFound, "profile_not_found", "profile was not found")
	case errors.Is(err, ErrSettingsRevisionConflict):
		writeAPIError(w, http.StatusConflict, "settings_changed", "settings changed in another tab; reload and try again")
	case errors.Is(err, ErrSettingsRecoveryRequired):
		writeAPIError(w, http.StatusServiceUnavailable, "settings_recovery_required", "settings require recovery; further changes are disabled")
	case errors.Is(err, ErrProfileMismatch):
		writeAPIError(w, http.StatusConflict, "profile_changed", "the active profile changed; reload and try again")
	case errors.Is(err, ErrSettingsBusy), errors.Is(err, ErrSendInProgress), errors.Is(err, ErrVoiceOperationConflict):
		writeAPIError(w, http.StatusConflict, "voice_conflict", "voice cannot be changed while another operation is in progress")
	case errors.Is(err, voice.ErrLocalModelDeleting):
		writeAPIError(w, http.StatusConflict, "voice_conflict", "the local speech model is being deleted")
	case errors.Is(err, ErrServiceStopped):
		writeAPIError(w, http.StatusServiceUnavailable, "server_stopping", "server is shutting down")
	case errors.Is(err, ErrInvalidVoiceSettings), errors.Is(err, store.ErrInvalidVoiceProvider), errors.Is(err, store.ErrInvalidVoiceSelection):
		writeAPIError(w, http.StatusBadRequest, "invalid_voice_settings", "voice provider and model selection is invalid")
	case errors.Is(err, ErrVoiceModelUnverified):
		writeAPIError(w, http.StatusUnprocessableEntity, "voice_model_unverified", "the selected model is not verified for speech-to-text")
	case errors.Is(err, voice.ErrTranscriptionModelsUnsupported):
		writeAPIError(w, http.StatusServiceUnavailable, "voice_models_unsupported", "the active provider cannot identify speech models")
	case errors.Is(err, voice.ErrLocalRuntimeUnavailable), errors.Is(err, voice.ErrLocalModelNotInstalled), errors.Is(err, voice.ErrLocalModelIntegrity), errors.Is(err, ErrVoiceNotReady):
		writeAPIError(w, http.StatusServiceUnavailable, "voice_not_ready", "voice is not configured and ready")
	default:
		writeAPIError(w, http.StatusBadGateway, "voice_error", fallback)
	}
}
