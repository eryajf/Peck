package app

import (
	"reflect"
	"testing"

	"github.com/eryajf/Peck/internal/core"
)

func TestQAColdStartRequestsRemainBoundedAndDoNotAuthorize(t *testing.T) {
	a := &application{}
	for i := 0; i < 8; i++ {
		link := &core.LinkRequest{Project: string(rune('A' + i))}
		a.receive(incoming{link: link})
	}
	if len(a.pending) != 5 || a.inspection != nil || a.trusted || a.state != pageIdle {
		t.Fatalf("cold-start queue escaped prepare-only contract: %+v", a)
	}
	a.receive(incoming{link: &core.LinkRequest{Project: "A"}})
	if len(a.pending) != 5 {
		t.Fatal("duplicate cold-start request expanded the queue")
	}
}

func TestQANewRequestPreservesCurrentConfirmation(t *testing.T) {
	original := &core.Inspection{Path: "/Applications/Original.app", Name: "Original"}
	current := &core.LinkRequest{Project: "Original"}
	a := &application{state: pageConfirm, inspection: original, link: current, trusted: true}
	a.receive(incoming{link: &core.LinkRequest{Project: "Other"}})
	if a.inspection != original || a.link != current || a.state != pageConfirm || !a.trusted || len(a.pending) != 1 {
		t.Fatal("incoming link replaced an existing confirmation")
	}
}

func TestQAProcessRequiresExplicitConfirmation(t *testing.T) {
	a := &application{state: pageConfirm, prefs: preferences{RequireTrust: true}, inspection: &core.Inspection{Path: "/untrusted.app"}}
	a.process()
	if a.state != pageConfirm || a.cancel != nil {
		t.Fatal("unconfirmed input started processing or enabled signing")
	}
	a.trusted = true
	a.inspection.BlockedReason = "blocked fixture"
	a.process()
	if a.state != pageConfirm || a.cancel != nil {
		t.Fatal("blocked application started processing")
	}
}

func TestQAInvalidPickerSelectionPreservesCurrentCard(t *testing.T) {
	states := []struct {
		name  string
		state page
	}{{"confirmation", pageConfirm}, {"result", pageResult}}
	selections := []struct {
		name  string
		paths []string
	}{
		{"cancelled", nil},
		{"empty_selection", []string{}},
		{"empty_path", []string{""}},
		{"ordinary_file", []string{"/Applications/notes.txt"}},
		{"disk_image", []string{"/Downloads/Example.dmg"}},
		{"multiple_apps", []string{"/Applications/One.app", "/Applications/Two.app"}},
		{"mixed_selection", []string{"/Applications/One.app", "/Downloads/notes.txt"}},
	}
	for _, state := range states {
		for _, selection := range selections {
			t.Run(state.name+"/"+selection.name, func(t *testing.T) {
				inspection := &core.Inspection{Path: "/Applications/Current.app", Name: "Current", CanSign: true}
				link := &core.LinkRequest{Project: "Current", BundleID: "org.example.current"}
				result := core.Result{Name: "Current", Signature: "未请求", Quarantine: "已移除", Launch: "未请求", Changed: 2, CanOpen: true}
				pending := []incoming{{path: "/Applications/Queued.app"}, {link: &core.LinkRequest{Project: "Queued"}}}
				a := &application{
					state: state.state, inspection: inspection, link: link, result: result,
					trusted: true, pending: append([]incoming(nil), pending...),
					notice: "Previous notice", generation: 7,
				}

				// This is the picker callback's behavior boundary, without opening
				// a native dialog or inspecting any actual application.
				a.selectPaths(selection.paths)

				if a.state != state.state || a.inspection != inspection || a.link != link {
					t.Fatal("cancelled or invalid picker selection replaced the current target")
				}
				if !reflect.DeepEqual(a.result, result) || !reflect.DeepEqual(a.pending, pending) {
					t.Fatal("cancelled or invalid picker selection discarded results or pending requests")
				}
				if !a.trusted || a.generation != 7 || a.cancel != nil {
					t.Fatal("cancelled or invalid picker selection reset confirmation or started work")
				}
				if len(selection.paths) == 0 {
					if a.notice != "Previous notice" {
						t.Fatal("cancelling the picker changed the existing notice")
					}
				} else if a.notice == "" || a.notice == "Previous notice" {
					t.Fatal("invalid selection did not provide an actionable input error")
				}
			})
		}
	}
}
