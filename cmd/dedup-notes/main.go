package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"noto/internal/config"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
)

func main() {
	noLLM := flag.Bool("no-llm", false, "skip LLM-based dedup, use word-overlap heuristic")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./cmd/dedup-notes [flags] <profile-slug>\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nProfiles are stored in ~/.noto/profiles/<slug>/\n")
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	slug := flag.Arg(0)

	if err := run(slug, *noLLM); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(slug string, noLLM bool) error {
	ctx := context.Background()

	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("db not found at %s: %w", dbPath, err)
	}

	db, err := store.OpenProfile(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	repo := store.NewMemoryNoteRepo(db)

	notes, err := listAllNotes(ctx, db)
	if err != nil {
		return fmt.Errorf("list notes: %w", err)
	}
	fmt.Printf("Found %d notes total\n", len(notes))
	if len(notes) == 0 {
		return nil
	}

	// Phase 1: exact-content dedup.
	exactDups := findExactDuplicates(notes)
	merged := mergeExactDuplicates(ctx, repo, exactDups)
	fmt.Printf("Exact duplicates merged: %d\n", merged)

	notes, err = listAllNotes(ctx, db)
	if err != nil {
		return fmt.Errorf("re-list notes: %w", err)
	}
	if len(notes) == 0 {
		fmt.Println("All notes were exact duplicates.")
		return nil
	}

	// Phase 2: semantic dedup.
	var mergedFuzzy int
	if noLLM {
		// Heuristic word-overlap fallback.
		groups := findFuzzyDuplicates(notes)
		mergedFuzzy = mergeFuzzyDuplicates(ctx, repo, groups)
	} else {
		// Load provider config — if available, use the LLM; otherwise fall back.
		meta, metaErr := profile.ReadMetadata(slug)
		if metaErr != nil {
			fmt.Fprintf(os.Stderr, "  warning: cannot read profile metadata: %v\n", metaErr)
		}
		var profileID string
		if meta != nil {
			profileID = meta.ID
		}

		adapter, model, err := loadExtractor(ctx, db, profileID)
		if err != nil || adapter == nil {
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: no LLM provider configured (%v), falling back to heuristic\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  warning: no LLM provider configured, falling back to heuristic\n")
			}
			groups := findFuzzyDuplicates(notes)
			mergedFuzzy = mergeFuzzyDuplicates(ctx, repo, groups)
		} else {
			mergedFuzzy = mergeLLMDuplicates(ctx, repo, adapter, model, notes)
		}
	}

	fmt.Printf("Semantic duplicates merged: %d\n", mergedFuzzy)

	remaining, _ := listAllNotes(ctx, db)
	fmt.Printf("Remaining notes after dedup: %d\n", len(remaining))
	return nil
}

// ---------------------------------------------------------------------------
// Provider / LLM
// ---------------------------------------------------------------------------

func loadExtractor(ctx context.Context, db *store.DB, profileID string) (provider.Adapter, string, error) {
	// Check env vars first (matches loadProviderConfig in chat_cmd.go).
	if apiKey := os.Getenv("NOTO_API_KEY"); apiKey != "" {
		model := os.Getenv("NOTO_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		endpoint := os.Getenv("NOTO_ENDPOINT")
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1"
		}
		adapter := provider.NewOpenAICompatible(provider.Config{
			ProviderType: "openai_compatible",
			Endpoint:     endpoint,
			Model:        model,
			APIKey:       apiKey,
		})
		return adapter, model, nil
	}

	if profileID == "" {
		return nil, "", errors.New("no profile ID")
	}

	cfgRepo := store.NewProviderConfigRepo(db)
	cfg, err := cfgRepo.GetActive(ctx, profileID)
	if err != nil {
		return nil, "", fmt.Errorf("get provider config: %w", err)
	}

	passphrase, err := security.MachinePassphrase()
	if err != nil {
		return nil, "", fmt.Errorf("passphrase: %w", err)
	}
	decrypted, err := security.Decrypt(cfg.CredentialRef, passphrase)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt key: %w", err)
	}

	model := cfg.EffectiveExtractorModel()
	adapter := provider.NewOpenAICompatible(provider.Config{
		ProviderType: "openai_compatible",
		Endpoint:     cfg.Endpoint,
		Model:        model,
		APIKey:       decrypted,
	})
	return adapter, model, nil
}

// ---------------------------------------------------------------------------
// LLM-based dedup
// ---------------------------------------------------------------------------

func mergeLLMDuplicates(ctx context.Context, repo *store.MemoryNoteRepo, adapter provider.Adapter, model string, notes []*store.MemoryNote) int {
	// Pre-cluster with low Jaccard threshold to minimize LLM calls.
	clusters := clusterNotes(notes, 0.35)
	totalMerged := 0

	for i, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}

		// Deduplicate within the cluster using the LLM.
		groups, err := llmClassifyCluster(ctx, adapter, model, cluster)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  cluster %d: LLM error: %v (skipping)\n", i+1, err)
			continue
		}

		for _, g := range groups {
			if len(g.merge) == 0 {
				continue
			}
			fmt.Printf("  LLM: keeping %q (imp=%d), merging %d note(s)\n",
				ellipsis(g.keep.Content, 55), g.keep.Importance, len(g.merge))
			for _, d := range g.merge {
				fmt.Printf("    -> %q (imp=%d)\n", ellipsis(d.Content, 55), d.Importance)
			}
			if err := mergeGroup(ctx, repo, g); err != nil {
				fmt.Fprintf(os.Stderr, "    merge failed: %v\n", err)
				continue
			}
			totalMerged += len(g.merge)
		}
	}
	return totalMerged
}

// llmClassifyCluster sends a cluster of notes to the LLM and returns
// groups of notes that the model considers duplicates.
func llmClassifyCluster(ctx context.Context, adapter provider.Adapter, model string, notes []*store.MemoryNote) ([]dupGroup, error) {
	if len(notes) < 2 {
		return nil, nil
	}

	// Build the prompt in German (user's notes are German).
	var sb strings.Builder
	sb.WriteString("Du bist ein Deduplizierungs-Assistent. Unten stehen mehrere Notizen.\n")
	sb.WriteString("Bestimme, welche Notizen dieselbe Information vermitteln (d.h. Duplikate sind)\n")
	sb.WriteString("und zusammengeführt werden sollten.\n\n")
	sb.WriteString("Notizen:\n")
	for _, n := range notes {
		fmt.Fprintf(&sb, "[%s] %s\n", n.ID, n.Content)
	}
	sb.WriteString("\nAntworte AUSSCHLIESSLICH mit einem JSON-Objekt im folgenden Format:\n")
	sb.WriteString(`{"duplicates": [{"keep": "ID", "merge": ["ID2", "ID3"]}]}` + "\n")
	sb.WriteString("Dabei ist 'keep' die ID der zu behaltenden Notiz und 'merge' die IDs der Duplikate,\n")
	sb.WriteString("die in die keep-Notiz eingefügt werden sollen.\n")
	sb.WriteString("Füge KEINE Notiz in 'merge' ein, die nicht semantisch äquivalent ist.\n")
	sb.WriteString("Wenn es keine Duplikate gibt, antworte mit: {\"duplicates\": []}\n")

	resp, err := adapter.Complete(ctx, provider.CompletionRequest{
		Model:       model,
		Messages:    []provider.Message{{Role: "user", Content: sb.String()}},
		MaxTokens:   2000,
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	return parseLLMResponse(resp.Content, notes)
}

func parseLLMResponse(content string, notes []*store.MemoryNote) ([]dupGroup, error) {
	// Extract JSON block.
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, errors.New("no JSON object in response")
	}
	content = content[start : end+1]

	var parsed struct {
		Duplicates []struct {
			Keep  string   `json:"keep"`
			Merge []string `json:"merge"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w\nraw: %s", err, content)
	}

	byID := make(map[string]*store.MemoryNote)
	for _, n := range notes {
		byID[n.ID] = n
	}

	var groups []dupGroup
	for _, d := range parsed.Duplicates {
		keep, ok := byID[d.Keep]
		if !ok {
			continue
		}
		var merge []*store.MemoryNote
		for _, mid := range d.Merge {
			if m, ok := byID[mid]; ok && mid != d.Keep {
				merge = append(merge, m)
			}
		}
		if len(merge) > 0 {
			keep, merge = pickBest(keep, merge)
			groups = append(groups, dupGroup{keep: keep, merge: merge})
		}
	}
	return groups, nil
}

// ---------------------------------------------------------------------------
// Clustering
// ---------------------------------------------------------------------------

type tokenNote struct {
	note  *store.MemoryNote
	token map[string]struct{}
}

func clusterNotes(notes []*store.MemoryNote, threshold float64) [][]*store.MemoryNote {
	sets := make([]tokenNote, len(notes))
	for i, n := range notes {
		sets[i] = tokenNote{note: n, token: tokenize(n.Content)}
	}

	var clusters [][]*store.MemoryNote
	assigned := make(map[string]bool)

	for i := range sets {
		if assigned[sets[i].note.ID] {
			continue
		}
		cluster := []*store.MemoryNote{sets[i].note}
		assigned[sets[i].note.ID] = true
		for j := i + 1; j < len(sets); j++ {
			if assigned[sets[j].note.ID] {
				continue
			}
			if jaccard(sets[i].token, sets[j].token) >= threshold {
				cluster = append(cluster, sets[j].note)
				assigned[sets[j].note.ID] = true
			}
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

// ---------------------------------------------------------------------------
// Heuristic fuzzy dedup (fallback)
// ---------------------------------------------------------------------------

func findFuzzyDuplicates(notes []*store.MemoryNote) []dupGroup {
	sets := make([]tokenNote, len(notes))
	for i, n := range notes {
		sets[i] = tokenNote{note: n, token: tokenize(n.Content)}
	}

	var groups []dupGroup
	merged := make(map[string]bool)

	for i := range sets {
		if merged[sets[i].note.ID] {
			continue
		}
		keepers := []*store.MemoryNote{sets[i].note}
		for j := i + 1; j < len(sets); j++ {
			if merged[sets[j].note.ID] {
				continue
			}
			if sets[i].note.Content == sets[j].note.Content {
				continue
			}
			sim := jaccard(sets[i].token, sets[j].token)
			if sim >= 0.55 {
				keepers = append(keepers, sets[j].note)
				merged[sets[j].note.ID] = true
			}
		}
		if len(keepers) > 1 {
			best, rest := pickBest(keepers[0], keepers[1:])
			groups = append(groups, dupGroup{keep: best, merge: rest})
		}
	}
	return groups
}

// ---------------------------------------------------------------------------
// Helpers (exact dedup, merge, tokenize, etc.)
// ---------------------------------------------------------------------------

type dupGroup struct {
	keep  *store.MemoryNote
	merge []*store.MemoryNote
}

func listAllNotes(ctx context.Context, db *store.DB) ([]*store.MemoryNote, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, profile_id, COALESCE(conversation_id,''), category, content, importance,
		       source_message_ids, created_at, updated_at
		FROM memory_notes
		ORDER BY importance DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var notes []*store.MemoryNote
	for rows.Next() {
		n := &store.MemoryNote{}
		var cat string
		if err := rows.Scan(
			&n.ID, &n.ProfileID, &n.ConversationID, &cat, &n.Content, &n.Importance,
			&n.SourceMessageIDs, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		n.Category = store.MemoryCategory(cat)
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func findExactDuplicates(notes []*store.MemoryNote) []dupGroup {
	byContent := make(map[string][]*store.MemoryNote)
	for _, n := range notes {
		key := strings.TrimSpace(n.Content)
		byContent[key] = append(byContent[key], n)
	}
	var groups []dupGroup
	for _, group := range byContent {
		if len(group) <= 1 {
			continue
		}
		best, rest := pickBest(group[0], group[1:])
		groups = append(groups, dupGroup{keep: best, merge: rest})
	}
	return groups
}

func mergeExactDuplicates(ctx context.Context, repo *store.MemoryNoteRepo, groups []dupGroup) int {
	count := 0
	for _, g := range groups {
		if err := mergeGroup(ctx, repo, g); err != nil {
			fmt.Fprintf(os.Stderr, "  failed to merge exact dup group (keep=%s): %v\n", g.keep.ID, err)
			continue
		}
		count += len(g.merge)
	}
	return count
}

func mergeFuzzyDuplicates(ctx context.Context, repo *store.MemoryNoteRepo, groups []dupGroup) int {
	count := 0
	for _, g := range groups {
		if g.keep == nil || len(g.merge) == 0 {
			continue
		}
		fmt.Printf("  heuristic: keeping %q (imp=%d), merging %d dupes\n",
			ellipsis(g.keep.Content, 50), g.keep.Importance, len(g.merge))
		for _, d := range g.merge {
			fmt.Printf("    -> %q (imp=%d)\n", ellipsis(d.Content, 50), d.Importance)
		}
		if err := mergeGroup(ctx, repo, g); err != nil {
			fmt.Fprintf(os.Stderr, "  failed to merge heuristic dup group (keep=%s): %v\n", g.keep.ID, err)
			continue
		}
		count += len(g.merge)
	}
	return count
}

func mergeGroup(ctx context.Context, repo *store.MemoryNoteRepo, g dupGroup) error {
	mergedIDs := g.keep.SourceMessageIDs
	for _, d := range g.merge {
		mergedIDs = mergeSourceJSON(mergedIDs, d.SourceMessageIDs)
		if d.Importance > g.keep.Importance {
			g.keep.Importance = d.Importance
		}
	}
	g.keep.SourceMessageIDs = mergedIDs
	if err := repo.Update(ctx, g.keep); err != nil {
		return fmt.Errorf("update keeper %s: %w", g.keep.ID, err)
	}
	for _, d := range g.merge {
		if err := repo.Delete(ctx, d.ID); err != nil {
			return fmt.Errorf("delete dupe %s: %w", d.ID, err)
		}
	}
	return nil
}

func pickBest(first *store.MemoryNote, rest []*store.MemoryNote) (*store.MemoryNote, []*store.MemoryNote) {
	all := append([]*store.MemoryNote{first}, rest...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Importance != all[j].Importance {
			return all[i].Importance > all[j].Importance
		}
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all[0], all[1:]
}

func mergeSourceJSON(a, b string) string {
	var as, bs []string
	_ = json.Unmarshal([]byte(a), &as)
	_ = json.Unmarshal([]byte(b), &bs)
	seen := make(map[string]bool)
	var merged []string
	for _, s := range append(as, bs...) {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		merged = append(merged, s)
	}
	out, _ := json.Marshal(merged)
	return string(out)
}

func tokenize(s string) map[string]struct{} {
	tokens := strings.Fields(strings.ToLower(s))
	stop := map[string]bool{
		"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true,
		"ein": true, "eine": true, "einen": true, "einem": true, "eines": true, "einer": true,
		"ist": true, "sind": true, "war": true, "waren": true, "wird": true, "werden": true,
		"hat": true, "haben": true, "hatte": true, "hatten": true,
		"kann": true, "kannst": true, "können": true, "konnte": true, "konnten": true,
		"muss": true, "müssen": true, "musste": true, "mussten": true,
		"will": true, "willst": true, "wollen": true, "wollte": true, "wollten": true,
		"und": true, "oder": true, "aber": true, "denn": true, "sondern": true,
		"nicht": true, "kein": true, "keine": true, "keinen": true,
		"auch": true, "noch": true, "schon": true, "erst": true, "mal": true,
		"mit": true, "von": true, "vom": true, "zu": true, "zum": true, "zur": true,
		"auf": true, "an": true, "in": true, "aus": true, "bei": true, "nach": true,
		"vor": true, "seit": true, "bis": true, "durch": true, "für": true, "gegen": true,
		"um": true, "über": true, "unter": true, "zwischen": true,
		"ich": true, "du": true, "er": true, "es": true,
		"wir": true, "ihr": true,
		"mich": true, "mir": true, "dich": true, "dir": true, "ihn": true, "ihm": true,
		"uns": true, "euch": true, "ihnen": true,
		"mein": true, "meine": true, "dein": true, "deine": true, "sein": true, "seine": true,
		"ihre": true, "unser": true, "unsere": true, "euer": true, "eure": true,
		"dieser": true, "diese": true, "dieses": true, "diesem": true, "diesen": true,
		"jener": true, "jene": true, "jenes": true,
		"dass": true, "was": true, "wer": true,
		"wie": true, "wann": true, "wo": true, "warum": true, "wieso": true, "weshalb": true,
		"sehr": true, "ganz": true, "etwas": true, "man": true, "alle": true, "jeder": true,
		"beide": true, "einige": true, "mehrere": true, "solche": true,
		"ja": true, "nein": true, "doch": true, "halt": true, "eben": true,
		"dann": true, "dort": true, "hier": true, "da": true, "hin": true, "her": true,
		"immer": true, "nie": true, "niemals": true, "oft": true, "manchmal": true,
		"wieder": true, "weiter": true, "endlich": true, "vielleicht": true,
	}
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		t = strings.Trim(t, ".,!?;:\"'()[]{}")
		if t == "" || stop[t] {
			continue
		}
		set[t] = struct{}{}
	}
	return set
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func ellipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
