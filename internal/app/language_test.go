package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/eryajf/Peck/internal/i18n"
)

func TestLanguageFromPreferencesFollowsSystemUntilChosen(t *testing.T) {
	if got, chosen := languageFromPreferences(preferences{}, []string{"zh-Hans-CN"}); got != i18n.Chinese || chosen {
		t.Fatalf("automatic language = %v, chosen=%t", got, chosen)
	}
	if got, chosen := languageFromPreferences(preferences{Language: "en"}, []string{"zh-CN"}); got != i18n.English || !chosen {
		t.Fatalf("saved language = %v, chosen=%t", got, chosen)
	}
}

func TestSetLanguagePersistsADeliberateChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Peck", "preferences.json")
	a := &application{prefs: preferences{Motion: true, Appearance: "system"}, prefsPath: path, language: i18n.Chinese, messages: i18n.For(i18n.Chinese)}
	a.setLanguage(i18n.English, true)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved preferences
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Language != "en" {
		t.Fatalf("saved language = %q, want en", saved.Language)
	}
}

func TestLanguagePreferenceInvalidValueFallsBackToSystem(t *testing.T) {
	if got, chosen := languageFromPreferences(preferences{Language: "fr"}, []string{"zh-CN"}); got != i18n.Chinese || chosen {
		t.Fatalf("invalid language = %v, chosen=%t", got, chosen)
	}
}
