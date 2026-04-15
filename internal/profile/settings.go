package profile

import (
	"errors"
	"time"

	"noto/internal/config"
)

// Settings describes mutable profile settings stored in profile.json.
type Settings struct {
	MemoryTokenBudget int
	UpdatedAt         time.Time
}

// ReadSettings returns normalized settings for a profile slug.
func ReadSettings(slug string) (*Settings, error) {
	meta, err := ReadMetadata(slug)
	if err != nil {
		return nil, err
	}
	return normalizeSettings(&Settings{
		MemoryTokenBudget: meta.MemoryTokenBudget,
		UpdatedAt:         meta.UpdatedAt,
	}), nil
}

// WriteSettings updates settings values in profile.json.
func WriteSettings(slug string, s *Settings) error {
	if s == nil {
		return errors.New("profile: settings is nil")
	}
	meta, err := ReadMetadata(slug)
	if err != nil {
		return err
	}
	settings := normalizeSettings(s)
	meta.MemoryTokenBudget = settings.MemoryTokenBudget
	meta.UpdatedAt = time.Now().UTC()
	return WriteMetadata(meta)
}

func normalizeSettings(s *Settings) *Settings {
	if s == nil {
		return &Settings{MemoryTokenBudget: config.DefaultMemoryTokenBudget}
	}
	if s.MemoryTokenBudget <= 0 {
		s.MemoryTokenBudget = config.DefaultMemoryTokenBudget
	}
	return s
}
