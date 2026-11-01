package app

import (
	"context"
	"strings"
	"testing"

	"github.com/eryajf/Peck/internal/core"
)

func TestLinkLookupIgnoresResultsAfterUserSelectionOrReset(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*application)
	}{
		{"reset", (*application).clear},
		{"manual picker", func(a *application) {
			a.cancelLinkLookup()
			a.dialog = true
		}},
		{"picker already open", func(a *application) { a.dialog = true }},
		{"another lookup", func(a *application) { a.generation++ }},
		{"manual inspection", func(a *application) { a.state = pageInspect }},
		{"confirmation", func(a *application) { a.state = pageConfirm }},
		{"processing", func(a *application) { a.state = pageProcessing }},
		{"result", func(a *application) { a.state = pageResult }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := &application{
				generation: 7, linkResolving: true,
				link:       &core.LinkRequest{Project: "Linked"},
				targetPath: "/Applications/Chosen.app",
				inspection: &core.Inspection{Name: "Chosen", Path: "/Applications/Chosen.app"},
				trusted:    true,
			}
			test.change(a)
			a.notice = "Keep current selection"
			state, inspection, path := a.state, a.inspection, a.targetPath
			trusted := a.trusted
			a.publish(wakeEvent{kind: wakeLinkTarget, generation: 7, linkPaths: []string{"/Applications/Linked.app"}})
			a.applyWakes()
			if a.state != state || a.inspection != inspection || a.targetPath != path ||
				a.trusted != trusted || a.notice != "Keep current selection" || a.cancel != nil {
				t.Fatal("late link lookup changed the current task or started an inspection")
			}
		})
	}
}

func TestLinkLookupMissingOrAmbiguousKeepsManualSelection(t *testing.T) {
	for _, paths := range [][]string{nil, {"/Applications/One.app", "/Applications/Two.app"}} {
		ctx, cancel := context.WithCancel(context.Background())
		a := &application{
			generation: 3, linkResolving: true, linkCancel: cancel,
			link: &core.LinkRequest{Project: "Example", BundleID: "org.example.app"},
		}
		a.publish(wakeEvent{kind: wakeLinkTarget, generation: 3, linkPaths: paths})
		a.applyWakes()
		if a.state != pageIdle || a.inspection != nil || a.targetPath != "" || a.trusted ||
			a.linkResolving || a.linkCancel != nil || ctx.Err() != context.Canceled {
			t.Fatal("lookup failed to return to manual selection")
		}
		if a.link == nil || !strings.Contains(a.notice, "选择应用") {
			t.Fatal("lookup did not retain project context and explain manual selection")
		}
	}
}

func TestClearingLinkLookupCancelsWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &application{linkResolving: true, linkCancel: cancel, link: &core.LinkRequest{Project: "Example"}}
	a.clear()
	if ctx.Err() != context.Canceled || a.linkResolving || a.linkCancel != nil || a.link != nil {
		t.Fatal("clearing the task left the automatic lookup running")
	}
}
