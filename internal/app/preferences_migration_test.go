package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPreferencesMigrateOnceAndKeepLegacyFile(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "PeckMac", "preferences.json")
	current := filepath.Join(dir, "Peck", "preferences.json")
	data := []byte(`{"motion":false,"appearance":"ocean","require_trust":true}`)
	writePreferenceFixture(t, legacy, data)

	want := preferences{Motion: false, Appearance: "ocean", RequireTrust: true}
	if got := readPreferencesFrom(dir); got != want {
		t.Fatalf("migration lost preferences: got %+v, want %+v", got, want)
	}
	for _, path := range []string{legacy, current} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("migration changed or lost %s: %s, %v", path, got, err)
		}
	}
	if info, err := os.Stat(current); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("migrated settings are not private: %v, %v", info, err)
	}

	writePreferenceFixture(t, legacy, []byte(`{"motion":true,"appearance":"rose"}`))
	if got := readPreferencesFrom(dir); got != want {
		t.Fatal("later legacy changes overwrote current preferences")
	}
}

func TestPreferenceMigrationNeverReplacesCurrentSettings(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "Peck", "preferences.json")
	data := []byte(`{"motion":true,"appearance":"sunset","require_trust":true}`)
	writePreferenceFixture(t, current, data)
	legacy := []byte(`{"motion":false,"appearance":"ocean"}`)
	writePreferenceFixture(t, filepath.Join(dir, "PeckMac", "preferences.json"), legacy)

	// Covers a new settings file appearing after the initial absence check.
	migratePreferences(current, legacy)
	if got := readPreferencesFrom(dir); got != (preferences{Motion: true, Appearance: "sunset", RequireTrust: true}) {
		t.Fatalf("new settings were replaced: %+v", got)
	}
	got, err := os.ReadFile(current)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("existing settings file changed: %s, %v", got, err)
	}
}

func TestPreferenceMigrationHandlesMissingAndInvalidFiles(t *testing.T) {
	for _, test := range []struct {
		name, legacy, current string
	}{
		{name: "missing"},
		{name: "invalid legacy", legacy: `{"motion":false,`},
		{name: "invalid current", legacy: `{"appearance":"ocean"}`, current: `{"motion":false,`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			current := filepath.Join(dir, "Peck", "preferences.json")
			if test.legacy != "" {
				writePreferenceFixture(t, filepath.Join(dir, "PeckMac", "preferences.json"), []byte(test.legacy))
			}
			if test.current != "" {
				writePreferenceFixture(t, current, []byte(test.current))
			}
			if got := readPreferencesFrom(dir); got != (preferences{Motion: true, Appearance: "system"}) {
				t.Fatalf("missing/invalid settings did not keep defaults: %+v", got)
			}
			if test.current == "" {
				if _, err := os.Stat(current); !os.IsNotExist(err) {
					t.Fatalf("invalid or missing legacy settings were migrated: %v", err)
				}
			}
		})
	}
}

func writePreferenceFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
