package config

import (
	"errors"
	"fmt"
	"os"
)

// DefaultMemoryTokenBudget is the default token budget for memory context selection.
const DefaultMemoryTokenBudget = 1500

const (
	defaultSystemPromptContent = "You are Noto. A buddy who takes notes."

	// DefaultExtractorPromptTemplate is the canonical default extractor prompt.
	// Keep this as the single source of truth used for file bootstrap and runtime fallback.
	DefaultExtractorPromptTemplate = `You are a memory extractor for a chat assistant.
Return ONLY valid JSON. No markdown. No code fences. No commentary.
Language policy: write note content in the same language as the user message.

Output schema (all keys required):
{
  "has_new_info": true|false,
  "confidence": 0.0-1.0,
  "notes": [
    {
      "action": "add|update",
      "target_id": "note_id_when_updating",
      "category": "fact|progress|blocker|action_item|other",
      "content": "max 220 chars, one concise sentence",
      "importance": 1-10
    }
  ]
}

Hard rules:
1) Always emit strict JSON (double quotes, no trailing commas).
2) Prioritize USER-provided information over assistant text.
   - Extract primarily from the user message.
   - Use assistant text only as context/confirmation, not as a source of new facts.
   - If user and assistant conflict, trust the user.
3) If nothing memory-worthy exists:
   - "has_new_info": false
   - "confidence": 0
   - "notes": []
4) Set "action" per note:
   - Use "update" only when clearly correcting/refining existing memory.
   - For "update", include a valid "target_id" from Existing notes.
   - For "add", set "target_id" to "".
5) Do not duplicate existing notes; prefer update when correcting, add when new.
6) Keep note content atomic and specific (no lists, no combined topics).

Importance rubric:
- 8-10: durable identity/long-term goals/major decisions likely useful across future sessions.
- 5-7: medium-term preferences, ongoing work context, recent important events.
- 1-4: minor or short-lived details.

Existing notes (use for update targeting):
%s

Exchange:
User: %s
Assistant: %s`
)

// PromptBootstrapResult captures whether prompt files were auto-created.
type PromptBootstrapResult struct {
	SystemCreated    bool
	ExtractorCreated bool
}

// MissingAny reports whether any prompt file was created during bootstrap.
func (r PromptBootstrapResult) MissingAny() bool {
	return r.SystemCreated || r.ExtractorCreated
}

// EnsureProfilePromptFiles ensures system/extractor prompt files exist for a profile.
// Missing files are created with defaults and reported in the result.
func EnsureProfilePromptFiles(slug string) (PromptBootstrapResult, error) {
	if slug == "" {
		return PromptBootstrapResult{}, errors.New("config: profile slug is required")
	}
	if err := EnsureAppDirs(slug); err != nil {
		return PromptBootstrapResult{}, err
	}

	systemPath, err := ProfileSystemPromptPath(slug)
	if err != nil {
		return PromptBootstrapResult{}, err
	}
	extractorPath, err := ProfileExtractorPromptPath(slug)
	if err != nil {
		return PromptBootstrapResult{}, err
	}

	res := PromptBootstrapResult{}
	if created, err := ensurePromptFile(systemPath, defaultSystemPromptContent); err != nil {
		return PromptBootstrapResult{}, err
	} else if created {
		res.SystemCreated = true
	}
	if created, err := ensurePromptFile(extractorPath, DefaultExtractorPromptTemplate); err != nil {
		return PromptBootstrapResult{}, err
	} else if created {
		res.ExtractorCreated = true
	}
	return res, nil
}

// ReadSystemPromptFile returns profile system prompt content, bootstrapping defaults if missing.
func ReadSystemPromptFile(slug string) (string, PromptBootstrapResult, error) {
	res, err := EnsureProfilePromptFiles(slug)
	if err != nil {
		return "", PromptBootstrapResult{}, err
	}
	path, err := ProfileSystemPromptPath(slug)
	if err != nil {
		return "", PromptBootstrapResult{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", PromptBootstrapResult{}, fmt.Errorf("config: read system prompt: %w", err)
	}
	return string(b), res, nil
}

// WriteSystemPromptFile writes profile system prompt markdown.
func WriteSystemPromptFile(slug, content string) error {
	if _, err := EnsureProfilePromptFiles(slug); err != nil {
		return err
	}
	path, err := ProfileSystemPromptPath(slug)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("config: write system prompt: %w", err)
	}
	return nil
}

// ReadExtractorPromptFile returns profile extractor prompt content, bootstrapping defaults if missing.
func ReadExtractorPromptFile(slug string) (string, PromptBootstrapResult, error) {
	res, err := EnsureProfilePromptFiles(slug)
	if err != nil {
		return "", PromptBootstrapResult{}, err
	}
	path, err := ProfileExtractorPromptPath(slug)
	if err != nil {
		return "", PromptBootstrapResult{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", PromptBootstrapResult{}, fmt.Errorf("config: read extractor prompt: %w", err)
	}
	return string(b), res, nil
}

// WriteExtractorPromptFile writes profile extractor prompt markdown.
func WriteExtractorPromptFile(slug, content string) error {
	if _, err := EnsureProfilePromptFiles(slug); err != nil {
		return err
	}
	path, err := ProfileExtractorPromptPath(slug)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("config: write extractor prompt: %w", err)
	}
	return nil
}

func ensurePromptFile(path, defaultContent string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("config: stat prompt file %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(defaultContent), 0o600); err != nil {
		return false, fmt.Errorf("config: bootstrap prompt file %s: %w", path, err)
	}
	return true, nil
}
