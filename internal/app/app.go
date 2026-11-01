// Package app owns Peck's Viva native UI. Business work remains in core;
// painting, input and lifecycle are driven by github.com/vivaui/viva/app.
package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/core"
	"github.com/eryajf/Peck/internal/i18n"
	"github.com/eryajf/Peck/internal/update"
	"github.com/eryajf/Peck/internal/version"
	"github.com/eryajf/Peck/internal/vui"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/app/singleinstance"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

const panelWidth = 400
const panelHeight = 440

// Viva treats an all-zero Color as "use the default opaque clear color". A
// tiny alpha keeps this explicitly transparent while preserving black RGB so
// the macOS compositor has no visible backdrop to blend at the rounded edge.
const transparentClearAlpha = 0.000001

func transparentClearColor() vapp.Color { return vapp.Color{Alpha: transparentClearAlpha} }

type page uint8

const (
	pageIdle page = iota
	pageInspect
	pageConfirm
	pageProcessing
	pageResult
)

type incoming struct {
	path string
	link *core.LinkRequest
}

// wakeKind identifies a result published by a background worker. Viva runs
// OnWake on the window thread immediately before Paint; workers only publish
// these typed values and never mutate application state directly.
type wakeKind uint8

const (
	wakeInspect wakeKind = iota + 1
	wakeProgress
	wakeProcess
	wakeChoose
	wakeOpen
	wakeLaunch
	wakeIcon
	wakeLinkTarget
	wakeUpdateCheck
	wakeUpdateDownload
)

type wakeEvent struct {
	kind           wakeKind
	generation     uint64
	inspection     *core.Inspection
	icon           vui.ImageSource
	result         core.Result
	progress       core.Progress
	paths          []string
	err            error
	openResult     core.Result
	openErr        error
	launchArgs     []string
	linkPaths      []string
	update         update.Result
	updateProgress update.Progress
	updatePath     string
}

var (
	tree                   = ui.Tree{}
	mainFeature            = tree.Feature("main")
	settingsFeature        = tree.Feature("settings")
	menuFeature            = tree.Feature("tray-menu")
	idChoose               = tree.ID(mainFeature, "choose")
	idSettings             = tree.ID(mainFeature, "settings")
	idHide                 = tree.ID(mainFeature, "hide")
	idStop                 = tree.ID(mainFeature, "stop")
	idBusy                 = tree.ID(mainFeature, "busy")
	idConfirm              = tree.ID(mainFeature, "confirm")
	idReplace              = tree.ID(mainFeature, "replace")
	idDone                 = tree.ID(mainFeature, "done")
	idOpen                 = tree.ID(mainFeature, "open")
	idRecheck              = tree.ID(mainFeature, "recheck")
	idTrust                = tree.ID(mainFeature, "trust")
	idTheme                = tree.ID(settingsFeature, "theme")
	idLanguage             = tree.ID(settingsFeature, "language")
	idAutoUpdate           = tree.ID(settingsFeature, "auto-update")
	idUpdateSource         = tree.ID(settingsFeature, "update-source")
	idProject              = tree.ID(settingsFeature, "project")
	idDonate               = tree.ID(settingsFeature, "donate")
	idCheckUpdate          = tree.ID(settingsFeature, "check-update")
	updateFeature          = tree.Feature("update")
	updateNotesLinks       = tree.Sub(updateFeature, tree.Name("notes-links"))
	idUpdatePromptLater    = tree.ID(updateFeature, "later")
	idUpdatePromptDownload = tree.ID(updateFeature, "download")
	idUpdatePromptRelease  = tree.ID(updateFeature, "release")
	idThemeScope           = tree.ID(settingsFeature, "theme-scope")
	idThemeDismiss         = tree.ID(settingsFeature, "theme-dismiss")
	idThemeScroll          = tree.ID(settingsFeature, "theme-scroll")
	themeOptionScope       = tree.Sub(settingsFeature, tree.Name("theme-option"))
	languageOptionScope    = tree.Sub(settingsFeature, tree.Name("language-option"))
	idMotion               = tree.ID(settingsFeature, "motion")
	idRequire              = tree.ID(settingsFeature, "require-trust")
	idBack                 = tree.ID(settingsFeature, "back")
	idQuit                 = tree.ID(settingsFeature, "quit")
	idMenuOpen             = tree.ID(menuFeature, "open")
	idMenuChoose           = tree.ID(menuFeature, "choose")
	idMenuSettings         = tree.ID(menuFeature, "settings")
	idMenuQuit             = tree.ID(menuFeature, "quit")
)

type application struct {
	host                                                     *vapp.Application
	panel, settings, trayMenu, updateWindow                  *vapp.Window
	tray                                                     *vapp.StatusItem
	mainInv, setInv, updateInv                               vapp.Invalidator
	state                                                    page
	inspection                                               *core.Inspection
	result                                                   core.Result
	progress                                                 core.Progress
	icon                                                     vui.ImageSource
	insectKind                                               int
	targetPath                                               string
	trusted, visible, dialog, opening, quitting, quitPending bool
	stopping, dropOver                                       bool
	themeOpen                                                bool
	// themePreview is a transient appearance selected by hovering the settings
	// popup. It is shared by both windows and is never persisted until the user
	// clicks an option.
	themePreview                         string
	themeScroll                          components.MenuScroll
	started, completed, motionStarted    time.Time
	launchStarted                        time.Time
	cancel                               context.CancelFunc
	generation                           uint64
	notice, shortcutError, settingsError string
	link                                 *core.LinkRequest
	pending                              []incoming
	prefs                                preferences
	language                             i18n.Language
	messages                             i18n.Messages
	prefsPath                            string
	mu                                   sync.Mutex
	wakes                                []wakeEvent
	stopAppearance                       func()
	targetGeneration                     uint64
	linkResolving                        bool
	linkCancel                           context.CancelFunc
	currentVersion                       string
	updateChecking                       bool
	updateResult                         update.Result
	updateError                          string
	updateCancel                         context.CancelFunc
	updateDownloadCancel                 context.CancelFunc
	updateStore                          *update.StateStore
	updateDownloading                    bool
	updateDownloadCancelRequested        bool
	updateProgress                       update.Progress
	updateReady                          update.ReadyState
	updateReadyOK                        bool
	updateNotes                          updateNotes
	updatePromptDismissedVersion         string
	updateCheckGeneration                uint64
	updateCheckAutomatic                 bool
	updateDownloadGeneration             uint64
	updateStartupReady                   update.ReadyState
}

func (a *application) ensureMessages() {
	if a.messages.SettingsTitle == "" {
		a.language = i18n.Chinese
		a.messages = i18n.For(a.language)
	}
}

func Run(args []string, currentVersion ...string) error {
	if runtime.GOOS != "darwin" {
		return errors.New(i18n.For(i18n.Chinese).AppOnlyMacOS)
	}
	if len(args) == 2 && args[0] == "--render-preview" {
		return renderPreview(args[1])
	}
	prefs := readPreferences()
	language, _ := languageFromPreferences(prefs, vapp.PreferredLanguages())
	buildVersion := version.Current
	if len(currentVersion) > 0 && strings.TrimSpace(currentVersion[0]) != "" {
		buildVersion = version.Normalize(currentVersion[0])
	}
	storePath := ""
	if dir, err := os.UserConfigDir(); err == nil {
		storePath = filepath.Join(dir, "Peck", "update-state.json")
	}
	a := &application{prefs: prefs, language: language, messages: i18n.For(language), currentVersion: buildVersion, updateStore: update.NewStateStore(storePath), insectKind: rand.IntN(brand.InsectCount), motionStarted: time.Now()}
	a.restoreUpdateState()
	claim, primary, err := singleinstance.SingleInstance("com.eryajf.peck", args, func(launchArgs []string) {
		a.publish(wakeEvent{kind: wakeLaunch, launchArgs: launchArgs})
	})
	if err == nil && !primary {
		return nil
	}
	if claim != nil {
		defer claim.Close()
	}
	defer unregisterGlobalShortcut()
	a.publish(wakeEvent{kind: wakeLaunch, launchArgs: args})
	return vapp.RunApplication(func(host *vapp.Application) error {
		a.host = host
		host.OnReopen(a.showMain)
		host.OnOpenURL(a.receiveURL)
		host.OnQuitRequest(func(allow func()) {
			a.cancelUpdateWork()
			if a.busy() || a.opening {
				a.quitting = true
				a.quitPending = true
				a.notice = a.messages.StoppingQuit
				a.stop()
				a.invalidate()
				return
			}
			a.quitting = true
			allow()
		})
		return a.ready()
	})
}

func trayIcon() vapp.StatusIcon {
	size, pix := brand.MenuBarPixels()
	return vapp.StatusIcon{Size: size, RGBA: pix, Template: true}
}

func windowIcon() vapp.WindowIcon {
	size, pix := brand.AppIconPixels()
	return vapp.WindowIcon{Size: size, RGBA: pix}
}

func (a *application) ready() error {
	a.ensureMessages()
	setAccessoryActivationPolicy()
	item, err := a.host.AddStatusItem(vapp.StatusItemConfig{
		Tooltip: a.messages.TrayTooltip, Icon: trayIcon(),
		// Viva's macOS status implementation makes an item menu-only when Menu
		// is set. Keep the primary press for the original show/hide toggle and
		// use a small WindowPanel for the secondary menu instead.
		OnClick: func(click vapp.StatusClick) {
			if click.Button == ui.PointerButtonSecondary {
				a.hide()
				a.openTrayMenu(click.Anchor)
				return
			}
			if a.visible {
				a.hide()
			} else {
				a.show()
			}
		},
		Menu: nil,
	})
	if err != nil && !errors.Is(err, vapp.ErrNoStatusArea) {
		return fmt.Errorf("创建菜单栏入口：%w", err)
	}
	a.tray = item
	if err := registerGlobalShortcut(func() {
		if a.visible {
			a.hide()
		} else {
			a.show()
		}
	}); err != nil {
		a.shortcutError = err.Error()
	}
	a.mainInv = vapp.Invalidator{}
	window, err := a.host.OpenWindow(vapp.Config{
		Title: "Peck", Width: panelWidth, Height: panelHeight, WindowMode: vapp.WindowFrameless,
		Icon:       windowIcon(),
		ClearColor: transparentClearColor(), Tree: &tree, Invalidator: &a.mainInv,
		Paint:  func(canvas *ui.Canvas, size ui.Size) { a.paintMain(canvas, size) },
		OnWake: func() { a.applyWakes() },
		OnKey:  a.mainKey,
		OnFileDrop: func(event ui.FileDropEvent) ui.EventResult {
			a.dropOver = event.Phase == ui.FileDropOver
			if event.Phase == ui.FileDropDrop {
				a.dropPaths(event.Paths)
			}
			return ui.EventResult{Handled: true, Repaint: true}
		}, OnShortcut: a.mainShortcut,
		OnCloseRequest: func(allow func()) {
			if !a.quitting {
				a.hide()
			} else {
				allow()
			}
		},
		OnClosed: func() {
			a.visible = false
			if a.stopAppearance != nil {
				a.stopAppearance()
				a.stopAppearance = nil
			}
		},
	}, nil)
	if err != nil {
		return err
	}
	a.panel = window
	a.stopAppearance = vapp.WatchSystemAppearance(func() {
		a.mainInv.Invalidate()
		if a.settings != nil {
			a.setInv.Invalidate()
		}
	})
	// The status item is optional on desktops without a status area; the panel
	// remains usable through the initial show and ordinary application launch.
	a.hide()
	a.mu.Lock()
	hasWakes := len(a.wakes) > 0
	a.mu.Unlock()
	if hasWakes {
		a.mainInv.Invalidate()
	}
	if len(a.pending) > 0 {
		a.next()
	} else {
		a.show()
	}
	if a.updateStartupReady.Path != "" {
		if err := update.ConfirmStartup(a.updateStartupReady.Path, a.updateStartupReady.Version, a.currentVersion); err == nil {
			_ = a.updateStore.ClearReady()
		}
	}
	if a.prefs.AutoCheckUpdate {
		a.checkForUpdate(false)
	}
	return nil
}

func (a *application) openTrayMenu(anchor vapp.StatusAnchor) {
	if a.host == nil {
		return
	}
	a.ensureMessages()
	if a.trayMenu != nil && !a.trayMenu.Closed() {
		a.trayMenu.Close()
		a.trayMenu = nil
	}
	window, err := a.host.OpenWindow(vapp.Config{
		Title: "Peck", Width: 228, Height: 188, WindowMode: vapp.WindowPanel, Anchor: anchor,
		ClearColor: transparentClearColor(), Tree: &tree,
		Paint:    func(canvas *ui.Canvas, size ui.Size) { a.paintTrayMenu(canvas, size) },
		OnClosed: func() { a.trayMenu = nil },
	}, nil)
	if err == nil {
		a.trayMenu = window
	}
}

func (a *application) paintTrayMenu(canvas *ui.Canvas, size ui.Size) {
	a.ensureMessages()
	bt := vivaTheme(a)
	panel := ui.Rect{X: 1, Y: 1, Width: size.Width - 2, Height: size.Height - 2}
	canvas.Shadow(panel, 14, ui.ShadowStyle{OffsetY: 4, Blur: 18}, vivaColor(bt.visuals.GlassGlow))
	canvas.RoundedBorder(panel, 14, 1, vivaColor(bt.visuals.GlassEdge), vivaColor(bt.visuals.BackgroundFrom))
	button := func(id ui.ID, y float32, label string, action func()) {
		r := ui.Rect{X: 12, Y: y, Width: size.Width - 24, Height: 34}
		canvas.RoundedBorder(r, 8, 1, vivaColor(bt.theme.Border), vivaColor(bt.theme.Surface))
		canvas.LabelCenter(label, r, ui.TextStyle{Size: 12, Weight: ui.WeightSemibold, Color: vivaColor(bt.theme.Text)})
		canvas.HitWithOptions(id, r, label, ui.HitOptions{Role: ui.RoleButton})
		if canvas.Clicked(id) {
			a.closeTrayMenu()
			action()
		}
	}
	button(idMenuOpen, 14, a.messages.MenuOpen, a.show)
	button(idMenuChoose, 54, a.messages.MenuChoose, func() { a.show(); a.choose() })
	button(idMenuSettings, 94, a.messages.MenuSettings, a.openSettings)
	button(idMenuQuit, 134, a.messages.MenuQuit, a.quit)
}

func (a *application) closeTrayMenu() {
	if a.trayMenu != nil {
		a.trayMenu.Close()
		a.trayMenu = nil
	}
}

func (a *application) quit() {
	a.cancelUpdateWork()
	a.closeTrayMenu()
	a.quitting = true
	if a.busy() || a.opening {
		a.quitPending = true
		a.stop()
		return
	}
	a.host.Quit()
}
func (a *application) mainShortcut(event ui.KeyEvent) ui.EventResult {
	if event.Modifiers&ui.ModifierMeta == 0 {
		return ui.EventResult{}
	}
	switch {
	case strings.EqualFold(event.Text, "w"):
		a.hide()
	case strings.EqualFold(event.Text, ","):
		a.openSettings()
	case strings.EqualFold(event.Text, "o"):
		a.choose()
	case strings.EqualFold(event.Text, "q"):
		a.quit()
	default:
		return ui.EventResult{}
	}
	return ui.EventResult{Handled: true, Repaint: true}
}

func (a *application) mainKey(event ui.KeyEvent) ui.EventResult {
	if event.Key != ui.KeyEscape {
		return ui.EventResult{}
	}
	a.hide()
	return ui.EventResult{Handled: true, Repaint: true}
}
func (a *application) busy() bool { return a.state == pageInspect || a.state == pageProcessing }
func (a *application) show() {
	if a.panel != nil {
		a.visible = true
		a.panel.Show()
		a.mainInv.Invalidate()
	}
}
func (a *application) showMain() {
	if a.quitting || a.quitPending {
		return
	}
	a.dismissThemeSelect()
	if a.settings != nil {
		a.settings.Hide()
	}
	a.closeTrayMenu()
	a.show()
}
func (a *application) hide() {
	a.visible = false
	if a.panel != nil {
		a.panel.Hide()
	}
}
func (a *application) invalidate() { a.mainInv.Invalidate() }

func (a *application) publish(event wakeEvent) {
	a.mu.Lock()
	if event.paths != nil {
		event.paths = append([]string(nil), event.paths...)
	}
	a.wakes = append(a.wakes, event)
	a.mu.Unlock()
	a.invalidate()
	a.setInv.Invalidate()
	a.updateInv.Invalidate()
}

// applyWakes is called on Viva's window thread. Every write to mutable UI
// state from a worker is applied here, preserving the framework's event-loop
// ownership rule.
func (a *application) applyWakes() {
	a.ensureMessages()
	a.mu.Lock()
	events := a.wakes
	a.wakes = nil
	a.mu.Unlock()
	for _, event := range events {
		switch event.kind {
		case wakeInspect:
			if event.generation != a.generation {
				continue
			}
			a.cancel = nil
			a.stopping = false
			if event.err != nil {
				a.state = pageIdle
				a.notice = fmt.Sprintf(a.messages.InspectFailed, a.messages.CoreText(event.err.Error()))
				if errors.Is(event.err, context.Canceled) {
					a.notice = a.messages.InspectCanceled
				}
				a.motionStarted = time.Now()
			} else {
				a.inspection = event.inspection
				a.targetPath = event.inspection.Path
				// Start with a deterministic fallback; a Darwin worker may replace it
				// with Finder's application artwork in a typed wake event. The icon
				// worker may win the race and publish first, so do not overwrite it.
				if a.icon == nil {
					a.icon = brand.AppIcon
				}
				a.state = pageConfirm
				a.trusted = !a.prefs.RequireTrust
			}
		case wakeIcon:
			// Icon extraction may finish after the user confirms or even starts
			// processing. Keep accepting it for the same selected target so the
			// launch animation never falls back to a pair of empty wings.
			if event.generation == a.targetGeneration && a.targetPath != "" && a.state != pageIdle && event.icon != nil {
				a.icon = event.icon
			}
		case wakeLinkTarget:
			if event.generation != a.generation || !a.linkResolving || a.state != pageIdle || a.dialog {
				continue
			}
			a.cancelLinkLookup()
			switch len(event.linkPaths) {
			case 1:
				// A unique Bundle ID match is only a candidate. Inspect it and
				// keep the normal confirmation step before any file changes.
				a.notice = ""
				a.inspect(event.linkPaths[0])
			case 0:
				a.notice = a.messages.LinkNoMatch
			default:
				a.notice = a.messages.LinkMultiple
			}
		case wakeProgress:
			if event.generation == a.generation && a.state == pageProcessing {
				a.progress = event.progress
			}
		case wakeProcess:
			if event.generation != a.generation {
				continue
			}
			a.cancel = nil
			a.result = event.result
			a.state = pageResult
			a.completed = time.Now()
			a.stopping = false
		case wakeChoose:
			a.dialog = false
			if event.err != nil && !errors.Is(event.err, vapp.ErrCanceled) {
				a.notice = event.err.Error()
			} else if event.err == nil {
				a.selectPaths(event.paths)
			}
		case wakeOpen:
			a.opening = false
			a.result = event.openResult
			if event.openErr != nil {
				a.launchStarted = time.Time{}
				a.notice = fmt.Sprintf(a.messages.OpenFailed, event.openErr.Error())
			}
		case wakeUpdateDownload:
			if event.generation != a.updateDownloadGeneration {
				continue
			}
			a.updateInv.Invalidate()
			if event.updateProgress.Received > 0 || event.updateProgress.Total > 0 {
				a.updateProgress = event.updateProgress
				a.setInv.Invalidate()
				continue
			}
			if a.updateDownloadCancelRequested {
				a.updateDownloading = false
				a.updateDownloadCancel = nil
				a.updateDownloadCancelRequested = false
				a.updateError = a.messages.UpdateDownloadCanceled
			} else if event.err != nil {
				a.updateDownloading = false
				a.updateDownloadCancel = nil
				if errors.Is(event.err, context.Canceled) {
					a.updateError = a.messages.UpdateDownloadCanceled
				} else {
					a.updateError = event.err.Error()
				}
			} else if event.updatePath != "" {
				a.updateDownloading = false
				a.updateDownloadCancel = nil
				a.updateError = ""
				a.updateReady = update.ReadyState{Version: event.update.LatestVersion, Path: event.updatePath}
				if event.update.Release.Asset != nil {
					a.updateReady.Asset = event.update.Release.Asset.Name
				}
				a.updateReadyOK = false
				if a.updateStore == nil {
					a.updateError = "update state store is unavailable"
				} else if err := a.updateStore.SaveReady(a.updateReady); err != nil {
					a.updateError = err.Error()
				} else {
					a.updateReadyOK = true
				}
			}
			a.setInv.Invalidate()
		case wakeUpdateCheck:
			if event.generation != a.updateCheckGeneration {
				continue
			}
			a.updateInv.Invalidate()
			a.updateChecking = false
			a.updateCheckAutomatic = false
			a.updateCancel = nil
			if event.err != nil {
				a.updateError = event.err.Error()
			} else {
				a.updateResult = event.update
				if a.updateStore != nil {
					_ = a.updateStore.SaveLastCheck(event.update)
				}
				if event.update.Comparison == update.ComparisonUpdateAvailable && event.update.LatestVersion != a.updatePromptDismissedVersion && !a.updateReadyOK {
					a.openUpdateWindow()
				}
			}
			a.setInv.Invalidate()
		case wakeLaunch:
			// An argument-free second instance still needs to raise the workspace.
			a.showMain()
			for _, arg := range event.launchArgs {
				if strings.HasPrefix(arg, core.LinkScheme+":") {
					a.receiveURL(arg)
				} else {
					a.receive(incoming{path: arg})
				}
			}
		}
		if a.quitPending && !a.busy() && !a.opening {
			a.host.Quit()
		}
	}
}

func (a *application) receiveURL(raw string) {
	a.ensureMessages()
	a.showMain()
	link, err := core.ParseLink(raw)
	if err != nil {
		a.notice = fmt.Sprintf(a.messages.InvalidLink, a.messages.CoreText(err.Error()))
		return
	}
	a.receive(incoming{link: &link})
}
func (a *application) receivePaths(paths []string) {
	a.ensureMessages()
	if len(paths) != 1 || paths[0] == "" {
		a.notice = a.messages.OneApp
		a.show()
		return
	}
	a.receive(incoming{path: paths[0]})
}
func (a *application) dropPaths(paths []string) {
	if a.busy() || a.opening || a.dialog {
		a.receivePaths(paths)
		return
	}
	a.selectPaths(paths)
}
func isApplicationPath(path string) bool {
	path = strings.TrimRight(path, string(os.PathSeparator))
	return strings.EqualFold(filepath.Ext(path), ".app")
}

func (a *application) receive(in incoming) {
	a.ensureMessages()
	isFile := in.path != "" || in.link == nil
	in.path = strings.TrimRight(in.path, string(os.PathSeparator))
	if isFile && !isApplicationPath(in.path) {
		a.notice = a.messages.InvalidApp
		a.show()
		return
	}
	if a.panel == nil || a.busy() || a.opening || a.dialog || a.state == pageConfirm || a.state == pageResult || in.link != nil && a.link != nil {
		for _, p := range a.pending {
			if p.path != "" && p.path == in.path || p.link != nil && in.link != nil && *p.link == *in.link {
				return
			}
		}
		if a.link != nil && in.link != nil && *a.link == *in.link {
			a.show()
			return
		}
		if a.inspection != nil && in.path == a.inspection.Path && a.state != pageResult {
			a.show()
			return
		}
		if len(a.pending) >= 5 {
			a.notice = a.messages.QueueFull
		} else {
			a.pending = append(a.pending, in)
			a.notice = a.messages.QueueRetained
		}
		a.show()
		return
	}
	a.accept(in)
}
func (a *application) accept(in incoming) {
	a.notice = ""
	if in.link != nil {
		a.clear()
		a.link = in.link
		if in.link.BundleID != "" {
			a.linkResolving = true
			a.generation++
			generation := a.generation
			link := *in.link
			ctx, cancel := context.WithCancel(context.Background())
			a.linkCancel = cancel
			go func() {
				defer cancel()
				a.publish(wakeEvent{
					kind:       wakeLinkTarget,
					generation: generation,
					linkPaths:  findLinkTargets(ctx, link),
				})
			}()
		}
		a.show()
		return
	}
	a.insectKind = (a.insectKind + 1 + rand.IntN(brand.InsectCount-1)) % brand.InsectCount
	a.inspect(in.path)
	a.show()
}
func (a *application) next() {
	if a.busy() || a.opening || a.dialog {
		return
	}
	a.clear()
	if len(a.pending) > 0 {
		in := a.pending[0]
		a.pending = a.pending[1:]
		a.accept(in)
	}
}
func (a *application) clear() {
	a.cancelLinkLookup()
	a.state = pageIdle
	a.inspection = nil
	a.icon = nil
	a.targetPath = ""
	a.trusted = false
	a.stopping = false
	a.link = nil
	a.notice = ""
	a.result = core.Result{}
	a.motionStarted = time.Now()
	a.launchStarted = time.Time{}
}

func (a *application) cancelLinkLookup() {
	if a.linkCancel != nil {
		a.linkCancel()
		a.linkCancel = nil
	}
	a.linkResolving = false
}

func (a *application) choose() {
	a.ensureMessages()
	if a.busy() || a.dialog || a.opening {
		return
	}
	a.cancelLinkLookup()
	a.dialog = true
	go func() {
		paths, err := vapp.OpenFiles(vapp.OpenFileOptions{Title: a.messages.ChooseDialog})
		a.publish(wakeEvent{kind: wakeChoose, paths: paths, err: err})
	}()
}
func (a *application) selectPaths(paths []string) {
	a.ensureMessages()
	if len(paths) == 0 {
		return
	}
	if len(paths) != 1 || !isApplicationPath(paths[0]) {
		a.receivePaths(paths)
		return
	}
	link := a.link
	a.clear()
	a.link = link
	a.receivePaths(paths)
}

func (a *application) inspect(path string) {
	a.cancelLinkLookup()
	a.state = pageInspect
	a.targetPath = path
	a.icon = nil
	a.trusted = !a.prefs.RequireTrust
	a.stopping = false
	a.launchStarted = time.Time{}
	a.started = time.Now()
	a.motionStarted = a.started
	a.generation++
	generation := a.generation
	a.targetGeneration = generation
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go func() {
		if icon := loadAppIcon(path); icon != nil {
			a.publish(wakeEvent{kind: wakeIcon, generation: generation, icon: icon})
		}
	}()
	go func() {
		inspection, err := core.Inspect(ctx, path)
		cancel()
		a.publish(wakeEvent{kind: wakeInspect, generation: generation, inspection: inspection, err: err})
	}()
}
func (a *application) process() {
	a.ensureMessages()
	if a.state != pageConfirm || a.inspection == nil || a.prefs.RequireTrust && !a.trusted || a.inspection.BlockedReason != "" || a.dialog {
		return
	}
	inspection := a.inspection
	a.state = pageProcessing
	a.started = time.Now()
	a.stopping = false
	a.notice = ""
	a.progress = core.Progress{Stage: "validate", Message: a.messages.ProgressValidate}
	a.generation++
	generation := a.generation
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go func() {
		// Confirmation authorizes quarantine removal, not re-signing.
		result := core.Process(ctx, inspection, core.Options{}, func(p core.Progress) {
			a.publish(wakeEvent{kind: wakeProgress, generation: generation, progress: p})
		})
		cancel()
		a.publish(wakeEvent{kind: wakeProcess, generation: generation, result: result})
	}()
}

func (a *application) stop() {
	if a.cancel != nil {
		a.stopping = true
		a.cancel()
		a.invalidate()
	}
}
func (a *application) openResult() {
	a.ensureMessages()
	if !a.result.CanOpen || a.opening || a.dialog || a.state != pageResult {
		return
	}
	a.opening = true
	a.launchStarted = time.Now()
	a.result.Launch = a.messages.LaunchRequest
	a.invalidate()
	result := a.result
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := core.Open(ctx, &result)
		a.publish(wakeEvent{kind: wakeOpen, openResult: result, openErr: err})
	}()
}
func (a *application) canRecheck() bool {
	if a.state != pageResult || a.busy() || a.opening || a.dialog || a.inspection == nil {
		return false
	}
	path := a.inspection.Path
	if !filepath.IsAbs(path) || !isApplicationPath(path) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}
func (a *application) recheck() {
	if !a.canRecheck() {
		return
	}
	path, link := a.inspection.Path, a.link
	a.clear()
	a.link = link
	a.inspect(path)
}
func (a *application) signatureLabel(status string) string {
	switch status {
	case "unsigned":
		return a.messages.SignatureUnsigned
	case "adhoc":
		return a.messages.SignatureAdhoc
	case "adhoc-invalid":
		return a.messages.SignatureAdhocInvalid
	case "valid":
		return a.messages.SignatureValid
	case "invalid":
		return a.messages.SignatureInvalid
	default:
		return a.messages.SignatureUnknown
	}
}
func (a *application) compactSignatureLabel(status string) string {
	switch status {
	case "adhoc":
		return a.messages.StatusSignatureAdhoc
	case "valid":
		return a.messages.StatusSignatureValid
	default:
		return a.signatureLabel(status)
	}
}
