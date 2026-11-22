package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"noto/internal/store"
)

// NewHandler returns the same-origin API and embedded asset handler for one server origin.
func NewHandler(service *Service, origin string, settingsBackends ...SettingsBackend) http.Handler {
	var settingsBackend SettingsBackend
	if len(settingsBackends) > 0 {
		settingsBackend = settingsBackends[0]
	}
	return NewHandlerWithVoice(service, origin, settingsBackend, nil)
}

func newHandler(service *Service, origin, expectedHost string, settingsBackend SettingsBackend, voiceBackend VoiceBackend) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		isAPI := strings.HasPrefix(r.URL.Path, "/api/")
		if isAPI {
			w.Header().Set("Cache-Control", "no-store")
		}
		if expectedHost == "" || r.Host != expectedHost {
			writeAPIError(w, http.StatusMisdirectedRequest, "invalid_host", "request host is not served")
			return
		}
		requestOrigin := r.Header.Get("Origin")
		if (requestOrigin != "" && requestOrigin != origin) || ((r.Method == http.MethodPost || r.Method == http.MethodDelete) && requestOrigin != origin) {
			writeAPIError(w, http.StatusForbidden, "invalid_origin", "request origin is not allowed")
			return
		}

		switch r.URL.Path {
		case "/api/state":
			if r.Method != http.MethodGet {
				writeMethodError(w, "GET")
				return
			}
			writeJSON(w, http.StatusOK, service.State())
		case "/api/settings":
			if settingsBackend == nil {
				writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
				return
			}
			switch r.Method {
			case http.MethodGet:
				profiles, err := settingsBackend.Profiles(r.Context())
				if err != nil {
					writeAPIError(w, http.StatusInternalServerError, "settings_unavailable", "settings are unavailable")
					return
				}
				activeProfileSlug := ""
				if service != nil {
					activeProfileSlug = service.State().Profile.Slug
				}
				writeJSON(w, http.StatusOK, SettingsDTO{ActiveProfileSlug: activeProfileSlug, Profiles: profiles})
			case http.MethodPost:
				handleSettingsSave(w, r, service, settingsBackend)
			default:
				writeMethodError(w, "GET, POST")
			}
		case "/api/settings/models":
			if r.Method != http.MethodGet {
				writeMethodError(w, "GET")
				return
			}
			if settingsBackend == nil {
				writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
				return
			}
			slug := r.URL.Query().Get("profile")
			if slug == "" {
				writeAPIError(w, http.StatusBadRequest, "invalid_profile", "profile is required")
				return
			}
			if service == nil {
				writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
				return
			}
			result, err := service.Models(r.Context(), settingsBackend, slug)
			if errors.Is(err, ErrSettingsProfileNotFound) {
				writeAPIError(w, http.StatusNotFound, "profile_not_found", "profile was not found")
				return
			}
			if err != nil {
				if errors.Is(err, ErrSettingsModelListUnavailable) {
					writeAPIError(w, http.StatusBadGateway, "models_unavailable", "models are unavailable")
				} else {
					writeAPIError(w, http.StatusInternalServerError, "settings_unavailable", "settings are unavailable")
				}
				return
			}
			writeJSON(w, http.StatusOK, result)
		case "/api/voice", "/api/voice/models", "/api/voice/transcriptions", "/api/voice/local-model", "/api/voice/local-model/download":
			if handleVoiceRoute(w, r, service, voiceBackend) {
				return
			}
		case "/api/messages":
			switch r.Method {
			case http.MethodGet:
				handleMessagesGet(w, r, service)
			case http.MethodPost:
				handleMessagePost(w, r, service)
			default:
				writeMethodError(w, "GET, POST")
			}
		case "/api/markdown":
			if r.Method != http.MethodPost {
				writeMethodError(w, "POST")
				return
			}
			handleMarkdownPreview(w, r)
		case "/api/memory":
			if r.Method != http.MethodGet {
				writeMethodError(w, "GET")
				return
			}
			handleMemoryGet(w, r, service)
		default:
			if isAPI {
				writeAPIError(w, http.StatusNotFound, "not_found", "API route not found")
				return
			}
			serveStatic(w, r)
		}
	})
}

func handleSettingsSave(w http.ResponseWriter, r *http.Request, service *Service, backend SettingsBackend) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "json_required", "request content type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		ProfileSlug     string  `json:"profile_slug"`
		Model           string  `json:"model"`
		EmbeddingsModel *string `json:"embeddings_model"`
		ExtractorModel  *string `json:"extractor_model"`
		Revision        uint64  `json:"revision"`
	}
	if err := decoder.Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "settings request exceeds 64 KiB")
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
		writeAPIError(w, http.StatusBadRequest, "invalid_settings", "profile_slug and nonzero revision are required")
		return
	}
	if service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
		return
	}
	var state StateDTO
	if saver, ok := backend.(ModelSettingsSaver); ok {
		state, err = service.SaveModelSettings(r.Context(), saver, request.ProfileSlug, ModelSettingsSelection{
			Model: request.Model, EmbeddingsModel: request.EmbeddingsModel, ExtractorModel: request.ExtractorModel,
		}, request.Revision)
	} else {
		if request.EmbeddingsModel != nil || request.ExtractorModel != nil {
			writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
			return
		}
		saver, ok := backend.(SettingsSaver)
		if !ok {
			writeAPIError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings are unavailable")
			return
		}
		state, err = service.SaveSettings(r.Context(), saver, request.ProfileSlug, request.Model, request.Revision)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrSettingsProfileNotFound):
			writeAPIError(w, http.StatusNotFound, "profile_not_found", "profile was not found")
		case errors.Is(err, ErrSettingsModelNotFound):
			writeAPIError(w, http.StatusBadRequest, "model_not_found", "model is not available for this profile")
		case errors.Is(err, ErrSettingsModelListUnavailable):
			writeAPIError(w, http.StatusBadGateway, "models_unavailable", "models are unavailable")
		case errors.Is(err, ErrSettingsRevisionConflict):
			writeAPIError(w, http.StatusConflict, "settings_changed", "settings changed in another tab; reload and try again")
		case errors.Is(err, ErrSettingsBusy), errors.Is(err, ErrSendInProgress), errors.Is(err, ErrProfileMismatch):
			writeAPIError(w, http.StatusConflict, "settings_conflict", "settings cannot be changed while the profile is busy")
		case errors.Is(err, ErrServiceStopped):
			writeAPIError(w, http.StatusServiceUnavailable, "server_stopping", "server is shutting down")
		case errors.Is(err, ErrSettingsRecoveryRequired):
			writeAPIError(w, http.StatusServiceUnavailable, "settings_recovery_required", "settings require recovery; further changes are disabled")
		default:
			writeAPIError(w, http.StatusInternalServerError, "settings_save_failed", "settings could not be saved")
		}
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func handleMarkdownPreview(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "json_required", "request content type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		Content string `json:"content"`
	}
	if err := decoder.Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "markdown request exceeds 64 KiB")
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
	writeJSON(w, http.StatusOK, struct {
		ContentHTML string `json:"content_html"`
	}{ContentHTML: RenderMarkdown(request.Content)})
}

func handleMessagesGet(w http.ResponseWriter, r *http.Request, service *Service) {
	query := r.URL.Query()
	limit, err := parseLimit(query.Get("limit"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
		return
	}
	var cursor *store.MessageHistoryCursor
	if encoded := query.Get("before"); encoded != "" {
		cursor, err = decodeCursor(encoded)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "history cursor is invalid")
			return
		}
	}
	page, err := service.Messages(r.Context(), cursor, limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "history_unavailable", "conversation history is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func handleMessagePost(w http.ResponseWriter, r *http.Request, service *Service) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "json_required", "request content type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		RequestID   string `json:"request_id"`
		Content     string `json:"content"`
		ProfileSlug string `json:"profile_slug"`
	}
	if err := decoder.Decode(&request); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "message request exceeds 64 KiB")
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
	if !requestIDPattern.MatchString(request.RequestID) || strings.TrimSpace(request.Content) == "" || request.ProfileSlug == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_message", "request_id, profile_slug, and non-empty content are required")
		return
	}
	response, err := service.SendForProfile(r.Context(), request.RequestID, request.Content, request.ProfileSlug)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		switch {
		case errors.Is(err, ErrProviderSetupRequired):
			writeAPIError(w, http.StatusServiceUnavailable, "provider_setup_required", "configure a provider before chatting")
		case errors.Is(err, ErrSettingsRecoveryRequired):
			writeAPIError(w, http.StatusServiceUnavailable, "settings_recovery_required", "settings require recovery; further changes are disabled")
		case errors.Is(err, ErrSendInProgress), errors.Is(err, ErrRequestIDConflict), errors.Is(err, ErrProfileMismatch):
			writeAPIError(w, http.StatusConflict, "send_conflict", "another message is in progress or request_id was reused")
		case errors.Is(err, ErrServiceStopped):
			writeAPIError(w, http.StatusServiceUnavailable, "server_stopping", "server is shutting down")
		default:
			writeAPIError(w, http.StatusBadGateway, "provider_error", "upstream request failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func handleMemoryGet(w http.ResponseWriter, r *http.Request, service *Service) {
	query := r.URL.Query()
	tab := query.Get("tab")
	if tab != "daily" && tab != store.SummaryTypeWeekly && tab != store.SummaryTypeMonthly {
		writeAPIError(w, http.StatusBadRequest, "invalid_tab", "tab must be daily, weekly, or monthly")
		return
	}
	limit, err := parseLimit(query.Get("limit"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
		return
	}
	offset := 0
	if raw := query.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_offset", "offset must be a nonnegative integer")
			return
		}
	}
	page, err := service.Memory(r.Context(), tab, offset, limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "memory_unavailable", "memory is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultPageSize, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxPageSize {
		return 0, errors.New("web: invalid page limit")
	}
	return limit, nil
}

func writeMethodError(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=()")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
}
