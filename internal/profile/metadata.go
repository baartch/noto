package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noto/internal/config"
)

const metadataFileName = "profile.json"

// ErrMetadataNotFound is returned when a profile metadata file is missing.
var ErrMetadataNotFound = errors.New("profile: metadata file not found")

// Metadata holds profile metadata stored within the profile directory.
type Metadata struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	SystemPromptPath  string    `json:"system_prompt_path"`
	MemoryTokenBudget int       `json:"memory_token_budget"`
}

// DefaultSystemPromptRelPath returns the prompt path relative to the profile directory.
func DefaultSystemPromptRelPath() string {
	return filepath.Join(config.PromptsDirName, config.SystemPromptName)
}

// DefaultSystemPromptPath returns the default absolute path for a profile's system prompt.
func DefaultSystemPromptPath(slug string) (string, error) {
	profileDir, err := config.ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, DefaultSystemPromptRelPath()), nil
}

// MetadataPath returns the absolute path to the metadata file for a profile slug.
func MetadataPath(slug string) (string, error) {
	profileDir, err := config.ProfileDir(slug)
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, metadataFileName), nil
}

// ReadMetadata reads the metadata file for a profile slug.
func ReadMetadata(slug string) (*Metadata, error) {
	path, err := MetadataPath(slug)
	if err != nil {
		return nil, err
	}
	return ReadMetadataFile(path)
}

// ReadMetadataFile reads the metadata file at the provided path.
func ReadMetadataFile(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMetadataNotFound
		}
		return nil, fmt.Errorf("profile: read metadata: %w", err)
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("profile: parse metadata: %w", err)
	}
	if err := validateMetadata(&meta); err != nil {
		return nil, err
	}
	return normalizeMetadata(&meta), nil
}

// WriteMetadata writes the profile metadata into the profile directory.
func WriteMetadata(meta *Metadata) error {
	if meta == nil {
		return errors.New("profile: metadata is nil")
	}
	if err := validateMetadata(meta); err != nil {
		return err
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = meta.CreatedAt
	}
	path, err := MetadataPath(meta.Slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("profile: create metadata dir: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("profile: marshal metadata: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("profile: write metadata: %w", err)
	}
	return nil
}

func validateMetadata(meta *Metadata) error {
	if strings.TrimSpace(meta.ID) == "" {
		return errors.New("profile: metadata id must not be empty")
	}
	if strings.TrimSpace(meta.Name) == "" {
		return errors.New("profile: metadata name must not be empty")
	}
	if strings.TrimSpace(meta.Slug) == "" {
		return errors.New("profile: metadata slug must not be empty")
	}
	if strings.TrimSpace(meta.SystemPromptPath) == "" {
		return errors.New("profile: metadata system prompt path must not be empty")
	}
	return nil
}

func normalizeMetadata(meta *Metadata) *Metadata {
	if meta == nil {
		return meta
	}
	if meta.MemoryTokenBudget <= 0 {
		meta.MemoryTokenBudget = config.DefaultMemoryTokenBudget
	}
	return meta
}

func removeMetadata(slug string) error {
	path, err := MetadataPath(slug)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("profile: remove metadata: %w", err)
	}
	return nil
}

func removeProfileDir(slug string) error {
	path, err := config.ProfileDir(slug)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("profile: remove profile dir: %w", err)
	}
	return nil
}
