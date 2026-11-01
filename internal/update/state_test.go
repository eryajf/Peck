package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateStorePersistsReadyPackage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Peck", "update-state.json")
	packagePath := filepath.Join(filepath.Dir(path), "Peck.zip")
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packagePath, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(path)
	want := ReadyState{Version: "0.2.0", Path: packagePath, Asset: "Peck.zip"}
	if err := store.SaveReady(want); err != nil {
		t.Fatal(err)
	}
	loaded, ok := NewStateStore(path).Ready()
	if !ok || loaded != want {
		t.Fatalf("loaded ready state = %#v, ok=%t", loaded, ok)
	}
}

func TestStateStoreConsumesHelperFailure(t *testing.T) {
	dir := t.TempDir()
	store := NewStateStore(filepath.Join(dir, "state.json"))
	pkg := filepath.Join(dir, "update.zip")
	if err := store.SaveReady(ReadyState{Version: "0.2.0", Path: pkg}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pkg+".error", []byte("helper failed"), 0600); err != nil {
		t.Fatal(err)
	}
	message, err := store.TakeApplyFailure()
	if err != nil || message != "helper failed" {
		t.Fatalf("failure = %q, %v", message, err)
	}
	message, err = store.TakeApplyFailure()
	if err != nil || message != "" {
		t.Fatalf("failure was not consumed: %q, %v", message, err)
	}
}

func TestStateStorePersistsLastCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Peck", "update-state.json")
	store := NewStateStore(path)
	want := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, CheckedAt: time.Now()}
	if err := store.SaveLastCheck(want); err != nil {
		t.Fatal(err)
	}
	loaded, ok := NewStateStore(path).LastCheck()
	if !ok || loaded.CurrentVersion != want.CurrentVersion || loaded.LatestVersion != want.LatestVersion || loaded.Comparison != want.Comparison {
		t.Fatalf("loaded state = %#v, ok=%t", loaded, ok)
	}
}
