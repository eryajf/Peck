package app

import (
	"reflect"
	"testing"

	"github.com/eryajf/Peck/internal/core"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestReopenReturnsToWorkspaceAndPreservesCurrentTask(t *testing.T) {
	for _, test := range []struct {
		name       string
		settings   bool
		mainHidden bool
		open       func(*application)
		wantQueued bool
	}{
		{"native with settings visible", true, false, (*application).showMain, false},
		{"native with settings hidden", false, true, (*application).showMain, false},
		{"native with main visible", false, false, (*application).showMain, false},
		{"argument-free second instance", true, true, func(a *application) {
			a.publish(wakeEvent{kind: wakeLaunch})
			a.applyWakes()
		}, false},
		{"system deeplink", true, false, func(a *application) {
			a.receiveURL("peck://prepare?v=1&project=Next")
		}, true},
		{"second instance deeplink", true, true, func(a *application) {
			a.publish(wakeEvent{kind: wakeLaunch, launchArgs: []string{"peck://prepare?v=1&project=Next"}})
			a.applyWakes()
		}, true},
		{"invalid deeplink", true, false, func(a *application) {
			a.receiveURL("peck://prepare?v=2&project=Next")
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, host := reopenTestApplication(t)
			current := &core.Inspection{Path: "/Applications/Current.app", Name: "Current"}
			link := &core.LinkRequest{Project: "Current"}
			pending := []incoming{{link: &core.LinkRequest{Project: "Queued"}}}
			a.state, a.inspection, a.link = pageConfirm, current, link
			a.trusted, a.themeOpen = true, true
			a.pending = append([]incoming(nil), pending...)
			if !test.settings {
				a.settings.Hide()
			}
			if test.mainHidden {
				a.hide()
			}
			test.open(a)
			if !a.visible || a.panel.Hidden() || !a.settings.Hidden() || host.front != "Peck" {
				t.Fatal("reopen did not leave the main workspace in front with settings hidden")
			}
			if a.themeOpen || a.state != pageConfirm || a.inspection != current || a.link != link || !a.trusted || a.cancel != nil {
				t.Fatal("reopen reset the active task or retained the settings popup")
			}
			if test.wantQueued {
				pending = append(pending, incoming{link: &core.LinkRequest{Project: "Next"}})
			}
			if !reflect.DeepEqual(a.pending, pending) {
				t.Fatalf("reopen changed the request queue: got %+v, want %+v", a.pending, pending)
			}
		})
	}
}

// Use real Viva Window state with a recording host, without launching an app
// or inspecting/modifying the fixture path.
func reopenTestApplication(t *testing.T) (*application, *reopenTestHost) {
	t.Helper()
	host := &reopenTestHost{}
	vapp.UseApplicationRunner(func(start func(vapp.Host) error) error { return start(host) })
	t.Cleanup(func() {
		for _, window := range host.windows {
			window.Close()
		}
		vapp.UseApplicationRunner(nil)
	})
	a := &application{visible: true}
	if err := vapp.RunApplication(func(application *vapp.Application) error {
		a.host = application
		var err error
		a.panel, err = application.OpenWindow(vapp.Config{Title: "Peck"}, nil)
		if err != nil {
			return err
		}
		a.settings, err = application.OpenWindow(vapp.Config{Title: "Peck settings"}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return a, host
}

type reopenTestHost struct {
	windows []*reopenTestWindow
	front   string
}

func (host *reopenTestHost) OpenWindow(config vapp.Config, _ *ui.Canvas, closed func()) (vapp.WindowControl, error) {
	window := &reopenTestWindow{host: host, title: config.Title, closed: closed}
	host.windows = append(host.windows, window)
	window.Show()
	return window, nil
}

func (*reopenTestHost) AddStatusItem(vapp.StatusItemConfig) (vapp.StatusItemControl, error) {
	return nil, vapp.ErrNoStatusArea
}

func (*reopenTestHost) SetDockBadge(string) {}

func (*reopenTestHost) Quit() {}

type reopenTestWindow struct {
	host   *reopenTestHost
	title  string
	closed func()
}

func (window *reopenTestWindow) Show() { window.host.front = window.title }
func (*reopenTestWindow) Hide()        {}
func (*reopenTestWindow) Frame() (ui.Rect, bool) {
	return ui.Rect{}, false
}
func (window *reopenTestWindow) Close() {
	if window.closed != nil {
		window.closed()
		window.closed = nil
	}
}
