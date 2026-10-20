package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"noto/internal/store"
)

func NewHandler(service *Service, origin string) http.Handler {
	expectedHost := ""
	if parsed, err := url.Parse(origin); err == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.User == nil {
		expectedHost = parsed.Host
	}
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
		if (requestOrigin != "" && requestOrigin != origin) || (r.Method == http.MethodPost && requestOrigin != origin) {
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
		case "/api/messages":
			switch r.Method {
			case http.MethodGet:
				handleMessagesGet(w, r, service)
			case http.MethodPost:
				handleMessagePost(w, r, service)
			default:
				writeMethodError(w, "GET, POST")
			}
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
		RequestID string `json:"request_id"`
		Content   string `json:"content"`
	}
	if err := decoder.Decode(&request); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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
	if !requestIDPattern.MatchString(request.RequestID) || strings.TrimSpace(request.Content) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_message", "request_id and non-empty content are required")
		return
	}
	response, err := service.Send(r.Context(), request.RequestID, request.Content)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		switch {
		case errors.Is(err, ErrProviderSetupRequired):
			writeAPIError(w, http.StatusServiceUnavailable, "provider_setup_required", "configure a provider before chatting")
		case errors.Is(err, ErrSendInProgress), errors.Is(err, ErrRequestIDConflict):
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
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
}
