package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"noto/internal/config"
	"noto/internal/store"
)

// DiscoveryWarning captures non-fatal issues encountered during discovery.
type DiscoveryWarning struct {
	Slug string
	Err  error
}

// DiscoverProfiles scans the profiles directory and reads metadata files.
func DiscoverProfiles() ([]*store.Profile, []DiscoveryWarning, error) {
	profilesDir, err := config.ProfilesDir()
	if err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*store.Profile{}, nil, nil
		}
		return nil, nil, fmt.Errorf("profile: read profiles dir: %w", err)
	}

	activeSlug := ""
	if active, err := config.ReadActiveProfile(); err == nil {
		activeSlug = active.Slug
	}

	profiles := make([]*store.Profile, 0, len(entries))
	warnings := []DiscoveryWarning{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		meta, err := ReadMetadata(slug)
		if err != nil {
			warnings = append(warnings, DiscoveryWarning{Slug: slug, Err: err})
			continue
		}
		if meta.Slug != slug {
			warnings = append(warnings, DiscoveryWarning{Slug: slug, Err: errors.New("metadata slug does not match directory")})
			continue
		}
		dbPath, err := config.ProfileDBPath(slug)
		if err != nil {
			return nil, nil, err
		}
		systemPromptPath := meta.SystemPromptPath
		if !filepath.IsAbs(systemPromptPath) {
			profileDir, err := config.ProfileDir(slug)
			if err != nil {
				return nil, nil, err
			}
			systemPromptPath = filepath.Join(profileDir, systemPromptPath)
		}
		p := &store.Profile{
			ID:               meta.ID,
			Name:             meta.Name,
			Slug:             meta.Slug,
			SystemPromptPath: systemPromptPath,
			DBPath:           dbPath,
			IsDefault:        meta.Slug == activeSlug,
			CreatedAt:        meta.CreatedAt,
			UpdatedAt:        meta.UpdatedAt,
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Name == profiles[j].Name {
			return profiles[i].Slug < profiles[j].Slug
		}
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, warnings, nil
}
