package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openVoiceSettingsTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := OpenProfile(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("open profile db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestVoiceSettingsRepoGetDefaultsAfterProfileMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.db")
	db, err := OpenProfile(path)
	if err != nil {
		t.Fatalf("open existing profile db: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `DROP TABLE voice_settings`); err != nil {
		t.Fatalf("remove new settings table to simulate a pre-voice profile: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `DELETE FROM schema_migrations WHERE version = '0006_voice_settings'`); err != nil {
		t.Fatalf("remove voice migration marker: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy profile db: %v", err)
	}

	db, err = OpenProfile(path)
	if err != nil {
		t.Fatalf("migrate existing profile db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := NewVoiceSettingsRepo(db)

	got, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("get voice settings: %v", err)
	}
	if got != (VoiceSettings{}) {
		t.Fatalf("default voice settings = %#v, want empty settings", got)
	}
}

func TestVoiceSettingsRepoSettingsArePerProfile(t *testing.T) {
	ctx := context.Background()
	first := NewVoiceSettingsRepo(openVoiceSettingsTestDB(t))
	second := NewVoiceSettingsRepo(openVoiceSettingsTestDB(t))

	firstSettings := VoiceSettings{Provider: "local"}
	secondSettings := VoiceSettings{Provider: "provider", Model: "vendor/stt-model"}
	if err := first.Save(ctx, firstSettings); err != nil {
		t.Fatalf("save first profile settings: %v", err)
	}
	if err := second.Save(ctx, secondSettings); err != nil {
		t.Fatalf("save second profile settings: %v", err)
	}

	for _, tc := range []struct {
		name string
		repo *VoiceSettingsRepo
		want VoiceSettings
	}{
		{name: "first", repo: first, want: firstSettings},
		{name: "second", repo: second, want: secondSettings},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.repo.Get(ctx)
			if err != nil {
				t.Fatalf("get voice settings: %v", err)
			}
			if got != tc.want {
				t.Fatalf("voice settings = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestVoiceSettingsRepoRejectsInvalidProvider(t *testing.T) {
	repo := NewVoiceSettingsRepo(openVoiceSettingsTestDB(t))

	err := repo.Save(context.Background(), VoiceSettings{Provider: "cloud"})
	if !errors.Is(err, ErrInvalidVoiceProvider) {
		t.Fatalf("save error = %v, want ErrInvalidVoiceProvider", err)
	}
}
