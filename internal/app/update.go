package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/update"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

var startUpdateApply = update.StartApply

const (
	projectURL = "https://github.com/eryajf/Peck"
	donateURL  = "https://wiki.eryajf.net/reward/"
)

func (a *application) restoreUpdateState() {
	if a.updateStore == nil {
		return
	}
	if failure, err := a.updateStore.TakeApplyFailure(); err == nil && strings.TrimSpace(failure) != "" {
		a.updateError = strings.TrimSpace(failure)
	}
	if saved, ok := a.updateStore.LastCheck(); ok && update.CompareVersions(a.currentVersion, saved.LatestVersion) == update.ComparisonUpdateAvailable {
		saved.CurrentVersion = a.currentVersion
		saved.Comparison = update.ComparisonUpdateAvailable
		saved.Release.URL = update.RewriteURL(saved.Release.URL, a.prefs.UpdateSource)
		if saved.Release.Asset != nil {
			asset := *saved.Release.Asset
			asset.DownloadURL = update.RewriteURL(asset.DownloadURL, a.prefs.UpdateSource)
			asset.ChecksumURL = update.RewriteURL(asset.ChecksumURL, a.prefs.UpdateSource)
			saved.Release.Asset = &asset
		}
		a.updateResult = saved
	}
	if ready, ok := a.updateStore.Ready(); ok {
		switch update.CompareVersions(a.currentVersion, ready.Version) {
		case update.ComparisonUpdateAvailable:
			a.updateReady, a.updateReadyOK = ready, true
		case update.ComparisonUpToDate:
			a.updateStartupReady = ready
		default:
			_ = a.updateStore.ClearReady()
		}
	} else {
		_ = a.updateStore.ClearReady()
	}
}

func (a *application) checkForUpdate(force bool) {
	if !force && !a.prefs.AutoCheckUpdate {
		return
	}
	if a.updateChecking || a.updateDownloading || a.quitting || a.quitPending {
		return
	}
	if force {
		a.updatePromptDismissedVersion = ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.updateCancel = cancel
	a.updateChecking = true
	if force {
		a.updateError = ""
	}
	a.setInv.Invalidate()
	a.updateInv.Invalidate()
	source, current := a.prefs.UpdateSource, a.currentVersion
	a.updateCheckGeneration++
	a.updateCheckAutomatic = !force
	generation := a.updateCheckGeneration
	go func() {
		defer cancel()
		result, err := update.Check(ctx, current, source, force)
		a.publish(wakeEvent{kind: wakeUpdateCheck, generation: generation, update: result, err: err})
	}()
}

// setUpdateSource invalidates in-flight checks; their late wake must not
// replace the new source. Already verified packages remain usable.
// setAutoCheckUpdate changes only the automatic-check preference. A disabled
// automatic check also invalidates an in-flight automatic worker so a late
// wake cannot publish a result after the user turned the setting off.
func (a *application) setAutoCheckUpdate(enabled bool) {
	if a.prefs.AutoCheckUpdate == enabled {
		return
	}
	a.prefs.AutoCheckUpdate = enabled
	if !enabled && a.updateChecking && a.updateCheckAutomatic {
		if a.updateCancel != nil {
			a.updateCancel()
			a.updateCancel = nil
		}
		a.updateCheckGeneration++
		a.updateCheckAutomatic = false
		a.updateChecking = false
	}
	a.savePreferences()
}

func (a *application) setUpdateSource(source string) {
	source = update.NormalizeSource(source)
	if source == update.NormalizeSource(a.prefs.UpdateSource) {
		return
	}
	if a.updateCancel != nil {
		a.updateCancel()
		a.updateCancel = nil
	}
	a.updateCheckGeneration++
	a.updateChecking = false
	a.prefs.UpdateSource = source
	a.updateResult = update.Result{}
	a.updateError = ""
	a.savePreferences()
	if a.host != nil && !a.updateDownloading && !a.quitting && !a.quitPending {
		a.checkForUpdate(true)
	}
}

func (a *application) cancelUpdateDownload() {
	if a.updateDownloadCancel != nil {
		a.updateDownloadCancelRequested = true
		a.updateDownloadCancel()
	}
}

func (a *application) cancelUpdateWork() {
	if a.updateCancel != nil {
		a.updateCancel()
	}
	a.cancelUpdateDownload()
}

func (a *application) downloadUpdate() {
	if a.updateDownloading || a.updateChecking || a.quitting || a.quitPending || a.updateResult.Comparison != update.ComparisonUpdateAvailable || update.CompareVersions(a.currentVersion, a.updateResult.LatestVersion) != update.ComparisonUpdateAvailable {
		return
	}
	asset := a.updateResult.Release.Asset
	if asset == nil {
		a.updateError = a.messages.UpdateNoAsset
		a.setInv.Invalidate()
		a.updateInv.Invalidate()
		return
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		a.updateError = err.Error()
		a.setInv.Invalidate()
		a.updateInv.Invalidate()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.updateDownloadCancel = cancel
	a.updateDownloadCancelRequested = false
	a.updateDownloading = true
	a.updateError = ""
	a.setInv.Invalidate()
	a.updateInv.Invalidate()
	result := a.updateResult
	a.updateDownloadGeneration++
	generation := a.updateDownloadGeneration
	go func() {
		defer cancel()
		path, err := update.Download(ctx, result, filepath.Join(dir, "Peck", "updates"), nil, func(progress update.Progress) {
			a.publish(wakeEvent{kind: wakeUpdateDownload, generation: generation, updateProgress: progress})
		})
		a.publish(wakeEvent{kind: wakeUpdateDownload, generation: generation, update: result, updatePath: path, err: err})
	}()
}

func (a *application) applyUpdate() {
	if a.host == nil || a.busy() || a.opening || a.dialog || a.quitting || a.quitPending || a.updateChecking || a.updateDownloading || !a.updateReadyOK || a.updateReady.Path == "" || update.CompareVersions(a.currentVersion, a.updateReady.Version) != update.ComparisonUpdateAvailable {
		return
	}
	if err := startUpdateApply(a.updateReady.Path); err != nil {
		a.updateError = err.Error()
		a.setInv.Invalidate()
		a.updateInv.Invalidate()
		return
	}
	a.quitting = true
	a.host.Quit()
}

const (
	updateWindowWidth  = 560
	updateWindowHeight = 440
)

// Release notes use the same source-owned Markdown view and scrolling recipe as
// Motty. The parsed document/layout are retained across progress repaints.
type updateNotes struct {
	doc                     markdown.Document
	view                    components.MarkdownView
	text                    string
	width                   float32
	height, offset, maximum float32
}

func (a *application) updateWindowConfig() vapp.Config {
	return vapp.Config{
		Title: "Peck · " + a.messages.Updates,
		Width: updateWindowWidth, Height: updateWindowHeight,
		MinWidth: 520, MinHeight: 400,
		WindowMode:     vapp.WindowCustomTitlebar,
		TitlebarHeight: settingsTitlebarHeight,
		Icon:           windowIcon(),
		ClearColor:     transparentClearColor(),
		Tree:           &tree, Invalidator: &a.updateInv,
		Paint:  a.paintUpdateWindow,
		OnWake: a.applyWakes,
		OnScroll: func(event ui.ScrollEvent) ui.EventResult {
			before := a.updateNotes.offset
			a.updateNotes.offset = min(max(before+event.DeltaY, 0), a.updateNotes.maximum)
			return ui.EventResult{Handled: true, Repaint: before != a.updateNotes.offset}
		},
		OnKey: func(event ui.KeyEvent) ui.EventResult {
			if event.Key == ui.KeyEscape {
				a.dismissUpdateWindow()
				return ui.EventResult{Handled: true}
			}
			step := float32(0)
			switch event.Key {
			case ui.KeyDown:
				step = 40
			case ui.KeyUp:
				step = -40
			case ui.KeyPageDown:
				step = 160
			case ui.KeyPageUp:
				step = -160
			}
			if step != 0 {
				a.updateNotes.offset = min(max(a.updateNotes.offset+step, 0), a.updateNotes.maximum)
				return ui.EventResult{Handled: true, Repaint: true}
			}
			return ui.EventResult{}
		},
		OnShortcut: func(event ui.KeyEvent) ui.EventResult {
			if event.Modifiers&ui.ModifierMeta != 0 && strings.EqualFold(event.Text, "w") {
				a.dismissUpdateWindow()
				return ui.EventResult{Handled: true}
			}
			return ui.EventResult{}
		},
		OnCloseRequest: func(func()) { a.dismissUpdateWindow() },
	}
}

func (a *application) openUpdateWindow() {
	a.ensureMessages()
	if a.host == nil || a.updateResult.Comparison != update.ComparisonUpdateAvailable || a.quitting || a.quitPending {
		return
	}
	if a.updateWindow != nil {
		a.updateWindow.Show()
		a.updateInv.Invalidate()
		return
	}
	a.updateInv = vapp.Invalidator{}
	config := a.updateWindowConfig()
	// Keep the independent window centered over an existing visible window;
	// without a visible owner Viva places it on the current screen.
	owner := a.settings
	if owner == nil || owner.Hidden() {
		owner = a.panel
	}
	if owner != nil && !owner.Hidden() {
		if frame, ok := owner.Frame(); ok {
			config.Origin = &ui.Point{X: frame.X + (frame.Width-updateWindowWidth)/2, Y: frame.Y + (frame.Height-updateWindowHeight)/2}
		}
	}
	var window *vapp.Window
	config.OnClosed = func() {
		if a.updateWindow == window {
			a.updateWindow = nil
		}
	}
	window, err := a.host.OpenWindow(config, nil)
	if err != nil {
		a.updateError = err.Error()
		return
	}
	a.updateWindow = window
	window.Show()
	a.updateInv.Invalidate()
}

func (a *application) dismissUpdateWindow() {
	a.updatePromptDismissedVersion = a.updateResult.LatestVersion
	if window := a.updateWindow; window != nil {
		a.updateWindow = nil
		window.Close()
	}
}

func (a *application) paintUpdateWindow(canvas *ui.Canvas, size ui.Size) {
	a.ensureMessages()
	appearance := a.effectiveAppearance()
	dark := appearance == "midnight" || appearance == "ocean"
	if appearance == "system" {
		dark = vapp.SystemAppearance() == vapp.AppearanceDark
	}
	t := brand.Theme(appearance, dark)
	v := brand.Visuals(appearance, dark)
	bt := &brandTheme{theme: t, visuals: v}
	canvas.FillLinearGradient(ui.Rect{Width: size.Width, Height: size.Height}, ui.LinearGradient{End: ui.Point{X: size.Width, Y: size.Height}, From: vivaColor(v.BackgroundFrom), To: vivaColor(v.BackgroundTo)})
	canvas.MoveRegion(ui.Rect{Width: size.Width, Height: settingsTitlebarHeight})
	leading, _ := vapp.TitlebarInsets()
	canvas.Label("Peck · "+a.messages.Updates, ui.Rect{X: leading + 14, Y: 7, Width: size.Width - leading - 28, Height: 20}, ui.TextStyle{Size: 13, Weight: ui.WeightSemibold, Color: vivaColor(t.Text)})
	canvas.Label(fmt.Sprintf(a.messages.UpdateAvailable, a.updateResult.LatestVersion), ui.Rect{X: 28, Y: 54, Width: size.Width - 56, Height: 28}, ui.TextStyle{Size: 20, Weight: ui.WeightBold, Color: vivaColor(t.Text)})
	canvas.Label("Peck "+a.currentVersion+" → "+a.updateResult.LatestVersion, ui.Rect{X: 28, Y: 92, Width: size.Width - 56, Height: 22}, ui.TextStyle{Size: 13, Color: vivaColor(t.TextMuted)})

	// Reserve the footer independently of release notes length and window size.
	box := ui.Rect{X: 28, Y: 128, Width: size.Width - 56, Height: max(80, size.Height-240)}
	canvas.RoundedBorder(box, 10, 1, vivaColor(t.Border), vivaColor(t.Surface))
	notes := &a.updateNotes
	text := a.updateResult.Release.Notes
	if strings.TrimSpace(text) == "" {
		text = a.messages.UpdateNotesEmpty
	}
	view := ui.Rect{X: box.X + 12, Y: box.Y + 10, Width: box.Width - 30, Height: box.Height - 20}
	if notes.text != text || notes.width != view.Width {
		notes.doc.Parse(text)
		notes.text, notes.width = text, view.Width
		notes.height = notes.view.Layout(canvas, components.MarkdownStyle{Size: 12, LineHeight: 20}, &notes.doc, view.Width)
	}
	scroller := components.BeginMessageScroller(canvas, settingsComponentTheme(t), view, notes.height, notes.offset)
	geometry := scroller.BeginContent()
	look := components.MarkdownLookFor(settingsComponentTheme(t))
	look.Link = vivaColor(t.Accent)
	notes.view.Paint(canvas, components.MarkdownPaint{Look: look, X: geometry.Content.X, Y: geometry.Content.Y, Clip: view, Links: updateNotesLinks})
	geometry = scroller.EndContent()
	notes.offset, notes.maximum = geometry.Offset, geometry.MaxOffset
	if index, ok := canvas.ClickedIn(updateNotesLinks); ok {
		a.openExternalURL(notes.view.Link(index))
	}

	status := a.updateStatusText()
	for index, line := range canvas.WrapLines(nil, status, 11, ui.WeightRegular, size.Width-56, 2) {
		canvas.Label(line, ui.Rect{X: 28, Y: box.Y + box.Height + 10 + float32(index*17), Width: size.Width - 56, Height: 17}, ui.TextStyle{Size: 11, Color: vivaColor(t.TextMuted)})
	}
	bottom := size.Height - 56
	if a.button(canvas, idUpdatePromptLater, ui.Rect{X: 28, Y: bottom, Width: 88, Height: 32}, a.messages.Later, false, false, bt) {
		a.dismissUpdateWindow()
	}
	if a.button(canvas, idUpdatePromptRelease, ui.Rect{X: 126, Y: bottom, Width: 120, Height: 32}, a.messages.OpenRelease, false, false, bt) {
		a.openExternalURL(a.updateResult.Release.URL)
	}
	label := a.messages.DownloadUpdate
	disabled := a.updateChecking || a.busy() || a.opening || a.dialog || a.quitting || a.quitPending
	if a.updateDownloading {
		label = a.messages.CancelDownload
		disabled = false
	} else if a.updateReadyOK {
		label = a.messages.ApplyUpdate
	}
	if a.button(canvas, idUpdatePromptDownload, ui.Rect{X: size.Width - 204, Y: bottom, Width: 176, Height: 32}, label, disabled, true, bt) {
		if a.updateDownloading {
			a.cancelUpdateDownload()
		} else if a.updateReadyOK {
			a.applyUpdate()
		} else {
			a.downloadUpdate()
		}
	}
}

func (a *application) openExternalURL(rawURL string) {
	if err := vapp.OpenURL(rawURL); err != nil {
		a.updateError = fmt.Sprintf(a.messages.OpenFailed, err)
		a.setInv.Invalidate()
		a.updateInv.Invalidate()
	}
}

func (a *application) updateStatusText() string {
	if a.updateChecking {
		return a.messages.CheckingUpdates
	}
	if a.updateDownloading {
		return fmt.Sprintf("%s %d/%d", a.messages.DownloadUpdate, a.updateProgress.Received, a.updateProgress.Total)
	}
	if a.updateError == a.messages.UpdateDownloadCanceled {
		return a.updateError
	}
	if a.updateError != "" {
		return fmt.Sprintf(a.messages.UpdateCheckFailed, a.updateError)
	}
	if a.updateReadyOK {
		return fmt.Sprintf(a.messages.UpdateAvailable, a.updateReady.Version)
	}
	switch a.updateResult.Comparison {
	case update.ComparisonUpdateAvailable:
		return fmt.Sprintf(a.messages.UpdateAvailable, a.updateResult.LatestVersion)
	case update.ComparisonUpToDate:
		return a.messages.UpToDate
	case update.ComparisonLocalNewer:
		return a.messages.LocalNewer
	case update.ComparisonUncomparable:
		return a.messages.UpdateUnavailable
	default:
		return ""
	}
}
