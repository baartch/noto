package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ActiveProfile stores instance-local active profile selection.
type ActiveProfile struct {
	Slug         string    `json:"slug"`
	LastSelected time.Time `json:"last_selected"`
}

// ErrActiveProfileNotFound is returned when no active profile is recorded.
var ErrActiveProfileNotFound = errors.New("config: active profile not set")

// ReadActiveProfile reads the active profile marker file.
func ReadActiveProfile() (*ActiveProfile, error) {
	path, err := ActiveProfilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrActiveProfileNotFound
		}
		return nil, fmt.Errorf("config: read active profile: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, ErrActiveProfileNotFound
	}
	ap := &ActiveProfile{Slug: strings.TrimSpace(lines[0])}
	if len(lines) > 1 {
		if ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(lines[1])); err == nil {
			ap.LastSelected = ts
		}
	}
	return ap, nil
}

// WriteActiveProfile writes the active profile marker file.
func WriteActiveProfile(slug string, lastSelected time.Time) error {
	path, err := ActiveProfilePath()
	if err != nil {
		return err
	}
	if strings.TrimSpace(slug) == "" {
		return errors.New("config: active profile slug must not be empty")
	}
	if lastSelected.IsZero() {
		lastSelected = time.Now().UTC()
	}
	content := fmt.Sprintf("%s\n%s\n", slug, lastSelected.UTC().Format(time.RFC3339Nano))
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: create app dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+ActiveProfileFile+"-*")
	if err != nil {
		return fmt.Errorf("config: create active profile temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: secure active profile temp file: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: write active profile: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: sync active profile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close active profile temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config: replace active profile: %w", err)
	}
	return nil
}

// ClearActiveProfile removes the active profile marker file.
func ClearActiveProfile() error {
	path, err := ActiveProfilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: remove active profile: %w", err)
	}
	return nil
}
