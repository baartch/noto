package web

import (
	"context"
	"errors"
)

// ErrSettingsProfileNotFound indicates the requested profile does not exist.
var ErrSettingsProfileNotFound = errors.New("web: settings profile not found")

// SettingsBackend provides profile-specific settings without mutating active settings.
type SettingsBackend interface {
	Profiles(context.Context) ([]SettingsProfileDTO, error)
	Models(context.Context, string) (ProfileModelsDTO, error)
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
