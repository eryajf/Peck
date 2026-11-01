package app

import (
	"testing"

	"github.com/eryajf/Peck/internal/core"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
)

func TestConfirmationKeepsStatusAndConsentControlsTogether(t *testing.T) {
	for _, test := range []struct {
		name                      string
		canSign, trust, processed bool
		fromLink                  bool
		blocked                   string
		wantTrust                 bool
	}{
		{name: "no optional rows"},
		{name: "trust only", trust: true, wantTrust: true},
		{name: "signature only", canSign: true},
		{name: "both options", canSign: true, trust: true, wantTrust: true},
		{name: "already processed", canSign: true, trust: true, processed: true, wantTrust: true},
		{name: "blocked", canSign: true, trust: true, blocked: "此应用的签名校验未通过，无法继续处理。请从可信来源重新获取应用后再次检查。"},
		{name: "local link target", fromLink: true},
		{name: "local link with both options", fromLink: true, canSign: true, trust: true, wantTrust: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := &application{
				state: pageConfirm, visible: true,
				prefs: preferences{Appearance: "rose", RequireTrust: test.trust},
				inspection: &core.Inspection{
					Name: "Example", Version: "1.0", SignatureStatus: "adhoc", QuarantineCount: 14,
					CanSign: test.canSign, AlreadyProcessed: test.processed, BlockedReason: test.blocked,
				},
			}
			if test.fromLink {
				a.link = &core.LinkRequest{Project: "Example", BundleID: "org.example.app"}
				a.inspection.Path = "/Applications/Example.app"
			}
			if test.name == "signature only" {
				a.inspection.SignatureStatus = "adhoc-invalid"
			}
			driver, err := vapp.NewDriver(vapp.Config{
				Width: panelWidth, Height: panelHeight, Tree: &tree, Paint: a.paintMain,
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			snapshot := driver.Snapshot()
			if test.fromLink {
				var found bool
				for _, text := range snapshot.Texts {
					if text.Value == "本机应用："+a.inspection.Path {
						found = true
					}
				}
				if !found {
					t.Fatal("link confirmation omitted the local application path")
				}
			}
			if problems := headless.CheckLayout(snapshot); len(problems) > 0 {
				t.Fatalf("confirmation layout problems: %+v", problems)
			}
			var gotTrust bool
			firstControlY := float32(panelHeight)
			for _, hit := range snapshot.Hits {
				switch hit.ID {
				case idTrust:
					gotTrust = true
				case idConfirm:
					if (test.trust || test.blocked != "") && !hit.Disabled {
						t.Fatal("layout change enabled an unconfirmed/blocked operation")
					}
					if test.name == "signature only" && hit.Disabled {
						t.Fatal("required signature repair should be included in confirmation")
					}
				default:
					continue
				}
				if hit.Bounds.Height <= 0 || hit.Bounds.Y+hit.Bounds.Height > panelHeight {
					t.Fatalf("consent/action is outside the panel: %+v", hit)
				}
				firstControlY = min(firstControlY, hit.Bounds.Y)
			}
			if gotTrust != test.wantTrust {
				t.Fatalf("visible trust option = %t", gotTrust)
			}
			if !test.processed && test.blocked == "" {
				for _, text := range snapshot.Texts {
					if text.Value == "隔离标记" {
						gap := firstControlY - (text.Bounds.Y + text.Bounds.Height)
						if gap < 8 || gap > 24 {
							t.Fatalf("status-to-action gap = %g px, want a compact separation", gap)
						}
					}
				}
			}
		})
	}
}
