package app

import (
	"path/filepath"
	"testing"

	"github.com/eryajf/Peck/internal/update"
)

func TestDisablingAutoCheckCancelsAndInvalidatesWorker(t *testing.T) {
	cancelled := false
	a := &application{
		prefs:                 preferences{AutoCheckUpdate: true},
		prefsPath:             filepath.Join(t.TempDir(), "preferences.json"),
		updateChecking:        true,
		updateCheckAutomatic:  true,
		updateCheckGeneration: 7,
		updateCancel:          func() { cancelled = true },
	}

	a.setAutoCheckUpdate(false)

	if !cancelled {
		t.Fatal("disabling automatic checks did not cancel the worker")
	}
	if a.updateCheckGeneration != 8 {
		t.Fatalf("generation = %d, want 8", a.updateCheckGeneration)
	}
	if a.updateChecking || a.updateCheckAutomatic || a.updateCancel != nil {
		t.Fatalf("automatic check remains active: checking=%v automatic=%v cancel=%v", a.updateChecking, a.updateCheckAutomatic, a.updateCancel != nil)
	}

	// A result from the canceled generation must be ignored even if the worker
	// managed to publish it before observing cancellation.
	a.publish(wakeEvent{kind: wakeUpdateCheck, generation: 7, update: update.Result{Comparison: update.ComparisonUpdateAvailable}})
	a.applyWakes()
	if a.updateResult.Comparison != "" {
		t.Fatal("late automatic result was accepted")
	}
}

func TestManualCheckIsNotCanceledWhenAutoCheckIsDisabled(t *testing.T) {
	cancelled := false
	a := &application{
		prefs:                 preferences{AutoCheckUpdate: true},
		prefsPath:             filepath.Join(t.TempDir(), "preferences.json"),
		updateChecking:        true,
		updateCheckAutomatic:  false,
		updateCheckGeneration: 7,
		updateCancel:          func() { cancelled = true },
	}

	// A manual check remains active when the preference is turned off.
	a.setAutoCheckUpdate(false)
	if cancelled || !a.updateChecking || a.updateCheckGeneration != 7 || a.updateCancel == nil {
		t.Fatal("disabling auto-check canceled or invalidated a manual check")
	}
}

func TestUpdateCheckPreventsReentry(t *testing.T) {
	cancelled := false
	a := &application{
		updateChecking:        true,
		updateCancel:          func() { cancelled = true },
		updateCheckGeneration: 3,
	}

	a.checkForUpdate(true)
	if a.updateCheckGeneration != 3 || cancelled {
		t.Fatal("reentrant update check replaced the active worker")
	}
}
