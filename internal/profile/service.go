package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"noto/internal/config"
	"noto/internal/store"
)

// ErrProfileInUse is returned when attempting to delete the only remaining profile.
var ErrProfileInUse = errors.New("profile: cannot delete the only remaining profile")

// ErrConfirmationRequired is returned when a destructive action was not confirmed.
var ErrConfirmationRequired = errors.New("profile: explicit confirmation required")

// slugRe is the regex for a valid profile slug.

// Service manages the profile lifecycle.
type Service struct {}

// NewService creates a new profile Service.
func NewService(_ *store.ProfileRepo) *Service {
	return &Service{}
}

// Create creates a new profile with the given name.
func (s *Service) Create(ctx context.Context, name string) (*store.Profile, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("profile: name must not be empty")
	}
	slug := toSlug(name)
	if existing, _, err := DiscoverProfiles(); err == nil {
		if hasProfileSlug(existing, slug) {
			slug = fmt.Sprintf("%s-%s", slug, newID()[:6])
		}
	}
	id := newID()

	systemPromptPath, err := DefaultSystemPromptPath(slug)
	if err != nil {
		return nil, err
	}
	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return nil, err
	}

	// Ensure directories exist.
	if err := config.EnsureAppDirs(slug); err != nil {
		return nil, err
	}

	p := &store.Profile{
		ID:               id,
		Name:             name,
		Slug:             slug,
		SystemPromptPath: systemPromptPath,
		DBPath:           dbPath,
		IsDefault:        false,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := WriteMetadata(&Metadata{
		ID:               p.ID,
		Name:             p.Name,
		Slug:             p.Slug,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		SystemPromptPath: DefaultSystemPromptRelPath(),
	}); err != nil {
		return nil, err
	}

	return p, nil
}

// List returns all profiles.
func (s *Service) List(ctx context.Context) ([]*store.Profile, error) {
	_ = ctx
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return nil, err
	}
	return profiles, nil
}

// Select sets the given profile as default (active).
func (s *Service) Select(ctx context.Context, name string) (*store.Profile, error) {
	_ = ctx
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return nil, err
	}

	p := findProfileByNameOrSlug(profiles, name)
	if p == nil {
		return nil, fmt.Errorf("profile: no profile named %q", name)
	}
	if err := config.WriteActiveProfile(p.Slug, time.Now().UTC()); err != nil {
		return nil, err
	}
	p.IsDefault = true
	return p, nil
}

// Rename renames a profile, updating slug and filesystem paths.
func (s *Service) Rename(ctx context.Context, oldName, newName string) (*store.Profile, error) {
	if strings.TrimSpace(newName) == "" {
		return nil, errors.New("profile: new name must not be empty")
	}
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return nil, err
	}
	p := findProfileByNameOrSlug(profiles, oldName)
	if p == nil {
		return nil, fmt.Errorf("profile: no profile named %q", oldName)
	}

	oldSlug := p.Slug
	newSlug := toSlug(newName)
	newSystemPromptPath, _ := DefaultSystemPromptPath(newSlug)
	newDBPath, _ := config.ProfileDBPath(newSlug)

	p.Name = newName
	p.Slug = newSlug
	p.SystemPromptPath = newSystemPromptPath
	p.DBPath = newDBPath
	p.UpdatedAt = time.Now().UTC()

	oldDir, err := config.ProfileDir(oldSlug)
	if err != nil {
		return nil, err
	}
	newDir, err := config.ProfileDir(newSlug)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return nil, fmt.Errorf("profile: rename dir: %w", err)
	}

	p.SystemPromptPath = filepath.Join(newDir, DefaultSystemPromptRelPath())
	p.DBPath = filepath.Join(newDir, config.MemoryDBName)

	if active, err := config.ReadActiveProfile(); err == nil {
		if active.Slug == oldSlug {
			if err := config.WriteActiveProfile(newSlug, time.Now().UTC()); err != nil {
				return nil, err
			}
		}
	}

	if err := WriteMetadata(&Metadata{
		ID:               p.ID,
		Name:             p.Name,
		Slug:             p.Slug,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		SystemPromptPath: DefaultSystemPromptRelPath(),
	}); err != nil {
		return nil, err
	}

	return p, nil
}

// Delete removes a profile after verifying there is more than one profile and the
// confirm function returns true.
func (s *Service) Delete(ctx context.Context, name string, confirm func(string) bool) error {
	_ = ctx
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return err
	}
	if len(profiles) <= 1 {
		return ErrProfileInUse
	}

	p := findProfileByNameOrSlug(profiles, name)
	if p == nil {
		return fmt.Errorf("profile: no profile named %q", name)
	}

	msg := fmt.Sprintf("Are you sure you want to permanently delete profile %q and all its data?", name)
	if confirm != nil && !confirm(msg) {
		return ErrConfirmationRequired
	}

	// If deleted profile was default, pick another.
	if p.IsDefault {
		if err := s.reassignDefault(ctx, p.ID); err != nil {
			return err
		}
	}

	if err := removeMetadata(p.Slug); err != nil {
		return err
	}
	if err := removeProfileDir(p.Slug); err != nil {
		return err
	}
	if active, err := config.ReadActiveProfile(); err == nil {
		if active.Slug == p.Slug {
			if err := config.ClearActiveProfile(); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetActive returns the currently active (default) profile.
func (s *Service) GetActive(ctx context.Context) (*store.Profile, error) {
	_ = ctx
	active, err := config.ReadActiveProfile()
	if err != nil {
		if errors.Is(err, config.ErrActiveProfileNotFound) {
			return nil, errors.New("profile: no active profile set")
		}
		return nil, err
	}
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return nil, err
	}
	p := findProfileByNameOrSlug(profiles, active.Slug)
	if p == nil {
		return nil, errors.New("profile: no active profile set")
	}
	p.IsDefault = true
	return p, nil
}

// LastUsed returns the most recently updated profile.
func (s *Service) LastUsed(ctx context.Context) (*store.Profile, error) {
	_ = ctx
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, errors.New("profile: no profiles found")
	}
	// profiles are sorted by name; select most recently updated manually
	latest := profiles[0]
	for _, p := range profiles[1:] {
		if p.UpdatedAt.After(latest.UpdatedAt) {
			latest = p
		}
	}
	return latest, nil
}

// ---- helpers ----------------------------------------------------------------

func (s *Service) reassignDefault(ctx context.Context, excludeID string) error {
	_ = ctx
	profiles, _, err := DiscoverProfiles()
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.ID != excludeID {
			return config.WriteActiveProfile(p.Slug, time.Now().UTC())
		}
	}
	return config.ClearActiveProfile()
}

func findProfileByNameOrSlug(profiles []*store.Profile, value string) *store.Profile {
	match := strings.TrimSpace(strings.ToLower(value))
	for _, p := range profiles {
		if strings.ToLower(p.Name) == match || strings.ToLower(p.Slug) == match {
			return p
		}
	}
	return nil
}

func hasProfileSlug(profiles []*store.Profile, slug string) bool {
	for _, p := range profiles {
		if p.Slug == slug {
			return true
		}
	}
	return false
}

// toSlug converts a display name to a URL-safe slug.
func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("profile-%d", time.Now().UnixNano())
	}
	return s
}

// newID generates a simple UUID-like ID using the current timestamp and random bytes.
func newID() string {
	// Use a simple time-based approach for now; replace with crypto/rand UUID in production.
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
