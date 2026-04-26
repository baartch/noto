package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"noto/internal/config"

	"golang.org/x/mod/semver"
)

// Result contains the outcome of an update check.
type Result struct {
	Current   string
	Latest    string
	HasUpdate bool
}

// Checker queries GitHub Releases and compares semantic versions.
type Checker struct {
	httpClient *http.Client
}

// NewChecker creates a Checker with a default 1s HTTP timeout.
func NewChecker() *Checker {
	return &Checker{httpClient: &http.Client{Timeout: 1 * time.Second}}
}

type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// Check returns whether a newer non-prerelease version is available.
func (c *Checker) Check(ctx context.Context, current string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.GitHubReleasesAPIURL, nil)
	if err != nil {
		return Result{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("update check failed: status %d", resp.StatusCode)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Result{}, err
	}

	latest := normalize(current)
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		tag := normalize(r.TagName)
		if !semver.IsValid(tag) || strings.Contains(tag, "-") {
			continue
		}
		if semver.Compare(tag, latest) > 0 {
			latest = tag
		}
	}

	curr := normalize(current)
	return Result{Current: curr, Latest: latest, HasUpdate: semver.Compare(latest, curr) > 0}, nil
}

func normalize(v string) string {
	if strings.TrimSpace(v) == "" {
		return "v0.0.0"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
