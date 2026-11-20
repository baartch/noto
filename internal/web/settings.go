package web

import (
	"context"
	"errors"

	"noto/internal/store"
)

// ErrSettingsProfileNotFound indicates the requested profile does not exist.
var ErrSettingsProfileNotFound = errors.New("web: settings profile not found")

// ErrSettingsModelListUnavailable indicates the provider's models endpoint failed.
var ErrSettingsModelListUnavailable = errors.New("web: settings model list unavailable")

// ErrSettingsRecoveryRequired blocks settings writes after failed compensation.
var ErrSettingsRecoveryRequired = errors.New("web: settings recovery required")

// SettingsBackend provides profile-specific settings without mutating active settings.
type SettingsBackend interface {
	Profiles(context.Context) ([]SettingsProfileDTO, error)
	Models(context.Context, string) (ProfileModelsDTO, error)
}

// SettingsSaver prepares a profile/model choice without publishing it.
type SettingsSaver interface {
	Prepare(context.Context, ProfileRuntime, string, string) (PreparedSettings, error)
}

// PreparedSettings commits or aborts a previously validated profile/model choice.
type PreparedSettings interface {
	Commit(context.Context) (ProfileRuntime, error)
	Abort()
}

// ErrSettingsRevisionConflict indicates settings changed after the editor opened.
var ErrSettingsRevisionConflict = errors.New("web: settings revision conflict")

// ErrSettingsBusy indicates a turn or memory extraction is using the active runtime.
var ErrSettingsBusy = errors.New("web: settings runtime is busy")

// ErrSettingsModelNotFound indicates the selected model is not available to the profile.
var ErrSettingsModelNotFound = errors.New("web: settings model not found")

// ErrProfileMismatch indicates a message request was prepared for another profile.
var ErrProfileMismatch = errors.New("web: message profile does not match active profile")

// ProfileRuntime holds the profile-scoped resources currently owned by the web service.
type ProfileRuntime struct {
	DB            *store.DB
	Profile       *store.Profile
	Session       ChatSession
	Model         string
	SetupRequired bool

	messageRepo messageHistoryReader
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.MemorySummaryRepo
}

type messageHistoryReader interface {
	ListHistoryByProfile(context.Context, string, *store.MessageHistoryCursor, int) (store.MessageHistoryPage, error)
}

// NewProfileRuntime creates the repositories and state for a profile runtime.
func NewProfileRuntime(db *store.DB, profile *store.Profile, session ChatSession, model string, setupRequired bool) ProfileRuntime {
	runtime := ProfileRuntime{DB: db, Session: session, Model: model, SetupRequired: setupRequired}
	if profile != nil {
		profileCopy := *profile
		runtime.Profile = &profileCopy
	}
	if db != nil {
		runtime.messageRepo = store.NewMessageRepo(db)
		runtime.noteRepo = store.NewMemoryNoteRepo(db)
		runtime.summaryRepo = store.NewMemorySummaryRepo(db)
	}
	return runtime
}

// SettingsProfileDTO contains a browser-safe profile choice.
type SettingsProfileDTO struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// SettingsDTO contains the active profile and available profile choices.
type SettingsDTO struct {
	ActiveProfileSlug string               `json:"active_profile_slug"`
	Profiles          []SettingsProfileDTO `json:"profiles"`
}

// ProfileModelsDTO contains the saved model and available models for one profile.
type ProfileModelsDTO struct {
	ProfileSlug   string   `json:"profile_slug"`
	SavedModel    string   `json:"saved_model"`
	Models        []string `json:"models"`
	SetupRequired bool     `json:"setup_required"`
}
