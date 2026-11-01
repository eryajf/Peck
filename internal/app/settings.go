package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/i18n"
	"github.com/eryajf/Peck/internal/vui"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

type preferences struct {
	Motion       bool   `json:"motion"`
	Appearance   string `json:"appearance"`
	RequireTrust bool   `json:"require_trust"`
	Language     string `json:"language,omitempty"`
}

const (
	settingsWidth          = 500
	settingsHeight         = 386
	settingsTitlebarHeight = 32
	themeRowHeight         = 32
	themeMenuRows          = 5
	themeRowInset          = float32(28)
	themeColumnGap         = float32(16)
)

type themeChoice struct {
	brand.AppearanceOption
	icon ui.Icon
}

// Build immutable swatches once; Viva caches their vector geometry when drawn.
var settingsThemes = func() []themeChoice {
	options := brand.AppearanceOptions()
	choices := make([]themeChoice, len(options))
	shape := []ui.PathCommand{
		ui.MoveTo(4, 0), ui.LineTo(12, 0), ui.QuadTo(16, 0, 16, 4),
		ui.LineTo(16, 12), ui.QuadTo(16, 16, 12, 16),
		ui.LineTo(4, 16), ui.QuadTo(0, 16, 0, 12),
		ui.LineTo(0, 4), ui.QuadTo(0, 0, 4, 0), ui.ClosePath(),
	}
	for index, option := range options {
		choices[index] = themeChoice{AppearanceOption: option, icon: ui.Icon{
			ViewBox: 16,
			Layers: []ui.IconLayer{{Commands: shape, Fill: vivaColor(option.AccentFrom), Gradient: &ui.LinearGradient{
				Start: ui.Point{Y: 8}, End: ui.Point{X: 16, Y: 8},
				From: vivaColor(option.AccentFrom), To: vivaColor(option.AccentTo),
			}}},
		}}
	}
	return choices
}()

func preferencesPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Peck", "preferences.json")
}
func readPreferences() preferences {
	dir, _ := os.UserConfigDir()
	return readPreferencesFrom(dir)
}

func readPreferencesFrom(dir string) preferences {
	p := preferences{Motion: true, Appearance: "system"}
	if dir == "" {
		return p
	}
	path := filepath.Join(dir, "Peck", "preferences.json")
	data, err := os.ReadFile(path)
	legacy := errors.Is(err, os.ErrNotExist)
	if legacy {
		data, err = os.ReadFile(filepath.Join(dir, "PeckMac", "preferences.json"))
	}
	if err != nil {
		return p
	}
	loaded := p
	if json.Unmarshal(data, &loaded) != nil {
		return p
	}
	loaded.Appearance = brand.NormalizeAppearance(loaded.Appearance)
	if legacy {
		migratePreferences(path, data)
	}
	return loaded
}

func languageFromPreferences(p preferences, preferred []string) (i18n.Language, bool) {
	if l, ok := i18n.FromTag(p.Language); ok {
		return l, true
	}
	return i18n.FromPreferredLanguages(preferred), false
}

func (a *application) setLanguage(language i18n.Language, persist bool) {
	a.language = language
	a.messages = i18n.For(language)
	if a.tray != nil {
		a.tray.SetTooltip(a.messages.TrayTooltip)
	}
	if persist {
		a.prefs.Language = language.String()
		a.savePreferences()
	}
	a.mainInv.Invalidate()
	a.setInv.Invalidate()
}

func (a *application) settingsSaveError(err error) string {
	if a.language == i18n.Chinese {
		if err == nil {
			return "无法找到设置目录。"
		}
		return "未能保存设置：" + err.Error()
	}
	if err == nil {
		return "Could not find the settings directory."
	}
	return "Could not save settings: " + err.Error()
}

// Publish a complete legacy copy only if no new preferences exist. Keeping
// the source allows an older development build to retain its own settings.
func migratePreferences(path string, data []byte) {
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".preferences-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		// Linking is atomic and cannot replace settings saved by another launch.
		_ = os.Link(file.Name(), path)
	}
}
func (a *application) savePreferences() {
	path := a.prefsPath
	if path == "" {
		path = preferencesPath()
	}
	defer a.setInv.Invalidate()
	defer a.mainInv.Invalidate()
	if path == "" {
		a.settingsError = a.settingsSaveError(nil)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		a.settingsError = err.Error()
		return
	}
	data, err := json.MarshalIndent(a.prefs, "", "  ")
	if err == nil {
		err = os.WriteFile(path+".tmp", data, 0600)
	}
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	if err != nil {
		a.settingsError = a.settingsSaveError(err)
	} else {
		a.settingsError = ""
	}
}

func (a *application) openSettings() {
	if a.host == nil {
		return
	}
	// Keep the main panel visible while settings is open so the appearance
	// preview can be compared against the real workspace. When the settings
	// window is first created, ask the native manager to place it beside the
	// panel; it will clamp the request to the available screen.
	if !a.visible {
		a.show()
	}
	if a.settings == nil {
		a.setInv = vapp.Invalidator{}
		config := a.settingsConfig()
		if a.panel != nil {
			if frame, ok := a.panel.Frame(); ok {
				point := ui.Point{X: frame.X + frame.Width + 16, Y: frame.Y}
				config.Origin = &point
			}
		}
		w, err := a.host.OpenWindow(config, nil)
		if err != nil {
			a.settingsError = err.Error()
			return
		}
		a.settings = w
	}
	a.settings.Show()
	a.setInv.Invalidate()
}

func (a *application) settingsConfig() vapp.Config {
	return vapp.Config{
		Title: a.messages.SettingsTitle, Width: settingsWidth, Height: settingsHeight, WindowMode: vapp.WindowCustomTitlebar,
		Icon:           windowIcon(),
		TitlebarHeight: settingsTitlebarHeight,
		ClearColor:     transparentClearColor(), Tree: &tree, Invalidator: &a.setInv,
		Paint: a.paintSettings, OnKey: a.settingsKey,
		OnScroll: a.settingsScroll, OnPointer: a.settingsPointer,
		OnShortcut: func(e ui.KeyEvent) ui.EventResult {
			if e.Modifiers&ui.ModifierMeta != 0 && strings.EqualFold(e.Text, "w") {
				a.closeSettings()
				return ui.EventResult{Handled: true, Repaint: true}
			}
			return ui.EventResult{}
		},
		OnMinimize: func(minimized bool) {
			if minimized {
				a.dismissThemeSelect()
				a.setInv.Invalidate()
			}
		},
		OnCloseRequest: func(allow func()) { a.closeSettings() },
		OnClosed: func() {
			a.dismissThemeSelect()
			a.settings = nil
		},
	}
}

func (a *application) closeSettings() {
	a.dismissThemeSelect()
	if a.settings != nil {
		a.settings.Hide()
	}
	if !a.quitting && !a.quitPending {
		a.show()
	}
}

func (a *application) settingsKey(e ui.KeyEvent) ui.EventResult {
	if a.themeOpen {
		switch e.Key {
		case ui.KeyEscape, ui.KeyTab:
			a.dismissThemeSelect()
			return ui.EventResult{Handled: true, Repaint: true, Focus: idTheme, FocusVisible: true, ContinueTraversal: e.Key == ui.KeyTab}
		}
		if id, ok := a.themeScroll.Navigate(e.Key, e.Target, true); ok {
			a.setThemePreview("")
			return ui.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}
		}
	} else if e.Target == idTheme && (e.Key == ui.KeyDown || e.Key == ui.KeyUp) {
		a.openThemeSelect()
		return ui.EventResult{Handled: true, Repaint: true, Focus: themeOptionScope.At(a.selectedTheme()), FocusVisible: true}
	} else if e.Target == idLanguage && (e.Key == ui.KeyLeft || e.Key == ui.KeyRight || e.Key == ui.KeyEnter || e.Key == ui.KeySpace) {
		if e.Key == ui.KeyLeft || e.Key == ui.KeyRight {
			if a.language == i18n.Chinese {
				a.setLanguage(i18n.English, true)
			} else {
				a.setLanguage(i18n.Chinese, true)
			}
		} else {
			a.setLanguage(a.language, true)
		}
		return ui.EventResult{Handled: true, Repaint: true, Focus: idLanguage, FocusVisible: true}
	} else if e.Key == ui.KeyEscape {
		a.closeSettings()
		return ui.EventResult{Handled: true, Repaint: true}
	}
	return ui.EventResult{}
}

func (a *application) selectedTheme() int {
	for index := range settingsThemes {
		if settingsThemes[index].Value == a.prefs.Appearance {
			return index
		}
	}
	return 0
}

func (a *application) openThemeSelect() {
	a.setThemePreview("")
	a.themeOpen = true
	a.themeScroll.BarID = idThemeScroll
	a.themeScroll.Offset = float32(max(0, a.selectedTheme()-themeMenuRows+1) * themeRowHeight)
}

func (a *application) paintSettings(canvas *ui.Canvas, size ui.Size) {
	a.ensureMessages()
	appearance := a.effectiveAppearance()
	dark := appearance == "midnight" || appearance == "ocean"
	if appearance == "system" {
		dark = vapp.SystemAppearance() == vapp.AppearanceDark
	}
	t := brand.Theme(appearance, dark)
	v := brand.Visuals(appearance, dark)
	canvas.FillLinearGradient(ui.Rect{Width: size.Width, Height: size.Height}, ui.LinearGradient{Start: ui.Point{}, End: ui.Point{X: size.Width, Y: size.Height}, From: vivaColor(v.BackgroundFrom), To: vivaColor(v.BackgroundTo)})
	r := ui.Rect{X: 8, Y: 8, Width: size.Width - 16, Height: size.Height - 16}
	canvas.RoundedBorder(r, appPanelCornerRadius, 1, vivaColor(v.GlassEdge), vivaColor(v.BackgroundFrom.Alpha(.9)))
	canvas.MoveRegion(ui.Rect{Width: size.Width, Height: settingsTitlebarHeight})
	leading, _ := vapp.TitlebarInsets()
	canvas.Label(a.messages.SettingsTitle, ui.Rect{X: leading + 14, Y: 7, Width: size.Width - leading - 28, Height: 20}, ui.TextStyle{Size: 13, Weight: ui.WeightSemibold, Color: vivaColor(t.Text)})
	const yShift = settingsTitlebarHeight
	a.paintAnimatedMark(canvas, brand.MarkSource(t.Dark), vui.Rect{X: 28, Y: 29 + yShift, W: 24, H: 27})
	canvas.Label(a.messages.SettingsDescription, ui.Rect{X: 62, Y: 28 + yShift, Width: size.Width - 90, Height: 30}, ui.TextStyle{Size: 20, Weight: ui.WeightBold, Color: vivaColor(t.Text)})
	labelBounds, _ := themeRowGeometry(size, yShift)
	languageLabel, languageTrigger := languageRowGeometry(size, yShift)
	canvas.Label(a.messages.Language, languageLabel, ui.TextStyle{Size: 14, Weight: ui.WeightSemibold, Color: vivaColor(t.Text)})
	a.paintLanguageSelect(canvas, languageTrigger, t)
	canvas.Label(a.messages.Theme, labelBounds, ui.TextStyle{Size: 14, Weight: ui.WeightSemibold, Color: vivaColor(t.Text)})
	a.paintThemeSelect(canvas, size, t, yShift)
	canvas.Fill(ui.Rect{X: 28, Y: 188 + yShift, Width: size.Width - 56, Height: 1}, vivaColor(t.Border.Alpha(.55)))
	y := 204 + yShift
	a.checkbox(canvas, idMotion, ui.Rect{X: 28, Y: float32(y), Width: size.Width - 56, Height: 27}, a.prefs.Motion, a.messages.Motion, false, &brandTheme{theme: t, visuals: v})
	a.checkbox(canvas, idRequire, ui.Rect{X: 28, Y: float32(y + 35), Width: size.Width - 56, Height: 27}, a.prefs.RequireTrust, a.messages.RequireTrust, false, &brandTheme{theme: t, visuals: v})
	shortcutLabel := a.messages.Shortcut
	if a.shortcutError != "" {
		shortcutLabel = a.messages.ShortcutUnavailable
	}
	canvas.Label(shortcutLabel, ui.Rect{X: 28, Y: 279 + yShift, Width: size.Width - 56, Height: 18}, ui.TextStyle{Size: 10, Color: vivaColor(t.TextMuted)})
	if a.button(canvas, idQuit, ui.Rect{X: size.Width - 180, Y: size.Height - 45, Width: 72, Height: 30}, a.messages.Quit, false, false, &brandTheme{theme: t, visuals: v}) {
		a.quit()
	}
	if a.button(canvas, idBack, ui.Rect{X: size.Width - 98, Y: size.Height - 45, Width: 70, Height: 30}, a.messages.Back, false, true, &brandTheme{theme: t, visuals: v}) {
		a.closeSettings()
	}
}

func languageRowGeometry(size ui.Size, yShift float32) (ui.Rect, ui.Rect) {
	left := themeRowInset
	right := size.Width - themeRowInset
	column := (right - left - themeColumnGap) / 2
	if column < 1 {
		column = 1
	}
	label := ui.Rect{X: left, Y: 80 + yShift, Width: column, Height: 36}
	trigger := ui.Rect{X: left + column + themeColumnGap, Y: label.Y, Width: column, Height: 36}
	return label, trigger
}

func (a *application) paintLanguageSelect(canvas *ui.Canvas, bounds ui.Rect, theme *vui.Theme) {
	canvas.RoundedBorder(bounds, 8, 1, vivaColor(theme.Border), vivaColor(theme.Surface))
	left := ui.Rect{X: bounds.X + 4, Y: bounds.Y + 4, Width: bounds.Width/2 - 6, Height: bounds.Height - 8}
	right := ui.Rect{X: bounds.X + bounds.Width/2 + 2, Y: bounds.Y + 4, Width: bounds.Width/2 - 6, Height: bounds.Height - 8}
	paint := func(bounds ui.Rect, label string, selected bool) {
		fill, color := theme.Surface, theme.TextMuted
		if selected {
			fill, color = theme.Accent, theme.AccentText
		}
		canvas.Rounded(bounds, 6, vivaColor(fill))
		canvas.LabelCenter(label, bounds, ui.TextStyle{Size: 10, Weight: ui.WeightMedium, Color: vivaColor(color)})
	}
	paint(left, a.messages.English, a.language == i18n.English)
	paint(right, a.messages.Chinese, a.language == i18n.Chinese)
	canvas.HitWithOptions(idLanguage, bounds, a.messages.Language, ui.HitOptions{Role: ui.RoleRadio, Checked: true})
	canvas.HitWithOptions(languageOptionScope.At(0), left, a.messages.English, ui.HitOptions{Role: ui.RoleRadio, Checked: a.language == i18n.English})
	canvas.HitWithOptions(languageOptionScope.At(1), right, a.messages.Chinese, ui.HitOptions{Role: ui.RoleRadio, Checked: a.language == i18n.Chinese})
	if canvas.Clicked(languageOptionScope.At(0)) {
		a.setLanguage(i18n.English, true)
	}
	if canvas.Clicked(languageOptionScope.At(1)) {
		a.setLanguage(i18n.Chinese, true)
	}
	if canvas.Clicked(idLanguage) {
		if a.language == i18n.Chinese {
			a.setLanguage(i18n.English, true)
		} else {
			a.setLanguage(i18n.Chinese, true)
		}
	}
}

func (a *application) effectiveAppearance() string {
	if a.themePreview != "" {
		return a.themePreview
	}
	return a.prefs.Appearance
}

func (a *application) previewThemeIndex() (int, bool) {
	if a.themePreview == "" {
		return 0, false
	}
	for index := range settingsThemes {
		if settingsThemes[index].Value == a.themePreview {
			return index, true
		}
	}
	return 0, false
}

func (a *application) settingsPointer(event ui.PointerEvent) ui.EventResult {
	preview := ""
	// Captured events identify the pressed item, which may no longer be under
	// the pointer. Stop previewing while dragging instead of retaining that
	// stale appearance after the pointer has left the option.
	if a.themeOpen && !event.Captured && event.Kind != ui.PointerExit {
		if index, ok := event.Target.In(themeOptionScope); ok && index >= 0 && index < len(settingsThemes) {
			preview = settingsThemes[index].Value
		}
	}
	changed := a.setThemePreview(preview)
	if a.themeOpen && (event.Target == idThemeScroll || a.themeScroll.Dragging()) {
		scrolled := a.themeScroll.Drag(event)
		return ui.EventResult{Handled: true, Repaint: changed || scrolled}
	}
	return ui.EventResult{Repaint: changed}
}

func (a *application) settingsScroll(event ui.ScrollEvent) ui.EventResult {
	if !a.themeOpen || !a.themeScroll.Wheel(event) {
		return ui.EventResult{Handled: true}
	}
	// Scrolling changes which item occupies the pointer's position. Wait for
	// Viva's next hover event rather than previewing a row that scrolled away.
	a.setThemePreview("")
	return ui.EventResult{Handled: true, Repaint: true}
}

func (a *application) setThemePreview(value string) bool {
	if a.themePreview == value {
		return false
	}
	a.themePreview = value
	a.applyNativeAppearance()
	// Settings repaints as part of its input event. The main window needs an
	// explicit wake even when its artwork animation is disabled.
	a.mainInv.Invalidate()
	return true
}

func (a *application) dismissThemeSelect() {
	a.themeOpen = false
	a.setThemePreview("")
}

// paintThemeSelect keeps the settings page compact while retaining a rich
// preview for every appearance. Viva owns the trigger, popup, clipping and
// selection indicator so text is painted once in its proper slot.
func (a *application) paintThemeSelect(canvas *ui.Canvas, size ui.Size, theme *vui.Theme, yShift float32) {
	selectedIndex := a.selectedTheme()
	selected := settingsThemes[selectedIndex]
	triggerChoice := selected
	if hoveredIndex, ok := a.previewThemeIndex(); ok {
		triggerChoice = settingsThemes[hoveredIndex]
	}
	_, trigger := themeRowGeometry(size, yShift)
	componentTheme := settingsComponentTheme(theme)
	content := ui.Rect{X: trigger.X, Y: trigger.Y + trigger.Height + 6, Width: trigger.Width, Height: themeMenuRows*themeRowHeight + 8}
	selectControl := components.BeginSelect(canvas, componentTheme, trigger, components.SelectOptions{
		TriggerID: idTheme, Label: a.messages.Theme, Value: a.themeLabel(triggerChoice.Value), Hint: a.themeDescription(triggerChoice.Value), Icon: triggerChoice.icon,
		Open: a.themeOpen, Content: content, Layer: 6, Scope: idThemeScope,
		DismissID: idThemeDismiss, Viewport: ui.Rect{Width: size.Width, Height: size.Height}, Scroll: &a.themeScroll,
	})
	if a.themeOpen {
		canvas.PushFocusScope(idThemeScope)
		canvas.SetInitialFocus(themeOptionScope.At(selectedIndex))
		canvas.PopFocusScope()
	}
	for index, option := range settingsThemes {
		selectControl.AddItem(components.SelectItem{
			ID: themeOptionScope.At(index), Label: a.themeLabel(option.Value), Value: option.Value,
			Hint: a.themeDescription(option.Value), Icon: option.icon, Selected: index == selectedIndex,
		})
	}
	selectControl.End()
	if index, ok := canvas.ClickedIn(themeOptionScope); ok && index >= 0 && index < len(settingsThemes) {
		a.setAppearance(settingsThemes[index].Value)
		a.dismissThemeSelect()
	}
	if canvas.Clicked(idThemeDismiss) {
		a.dismissThemeSelect()
	}
	if canvas.Clicked(idTheme) {
		a.openThemeSelect()
	}
}

func (a *application) themeLabel(value string) string {
	if s := a.messages.ThemeLabels[value]; s != "" {
		return s
	}
	for _, choice := range settingsThemes {
		if choice.Value == value {
			return choice.Label
		}
	}
	return value
}

func (a *application) themeDescription(value string) string {
	if s := a.messages.ThemeDescriptions[value]; s != "" {
		return s
	}
	for _, choice := range settingsThemes {
		if choice.Value == value {
			return choice.Description
		}
	}
	return ""
}

// themeRowGeometry gives the settings form two equal columns. The previous
// 120/300 split left too much visual weight on the selector, even though both
// outer margins were technically equal.
func themeRowGeometry(size ui.Size, yShift float32) (ui.Rect, ui.Rect) {
	left := themeRowInset
	right := size.Width - themeRowInset
	column := (right - left - themeColumnGap) / 2
	if column < 1 {
		column = 1
	}
	label := ui.Rect{X: left, Y: 124 + yShift, Width: column, Height: 36}
	trigger := ui.Rect{X: left + column + themeColumnGap, Y: label.Y, Width: column, Height: 36}
	return label, trigger
}

func settingsComponentTheme(theme *vui.Theme) ui.Theme {
	base := ui.LightTheme()
	if theme.Dark {
		base = ui.DarkTheme()
	}
	base.Background = vivaColor(theme.Background)
	base.Foreground = vivaColor(theme.Text)
	base.Card = vivaColor(theme.Surface)
	base.CardForeground = vivaColor(theme.Text)
	base.CardElevated = vivaColor(theme.SurfaceHover)
	base.Popover = vivaColor(theme.Surface)
	base.PopoverForeground = vivaColor(theme.Text)
	base.Muted = vivaColor(theme.SurfacePressed)
	base.MutedForeground = vivaColor(theme.TextMuted)
	base.Border = vivaColor(theme.Border)
	base.Hover = vivaColor(theme.SurfaceHover)
	base.Input = vivaColor(theme.Border)
	base.Primary = vivaColor(theme.Accent)
	base.PrimaryForeground = vivaColor(theme.AccentText)
	base.Secondary = vivaColor(theme.Surface)
	base.SecondaryForeground = vivaColor(theme.Text)
	base.Accent = vivaColor(theme.SurfaceHover)
	base.AccentForeground = vivaColor(theme.Text)
	base.Ring = vivaColor(theme.Focus)
	base.Success = vivaColor(theme.Success)
	base.Danger = vivaColor(theme.Danger)
	base.DangerForeground = vivaColor(theme.InverseText)
	base.Metrics = ui.DefaultMetrics()
	base.Metrics.FontSizeM = 12
	base.Metrics.FontSizeS = 11
	base.Metrics.RadiusControl = 8
	return base
}

func (a *application) setAppearance(value string) {
	a.themePreview = ""
	a.prefs.Appearance = brand.NormalizeAppearance(value)
	a.applyNativeAppearance()
	a.savePreferences()
}

func (a *application) applyNativeAppearance() {
	switch a.effectiveAppearance() {
	case "midnight", "ocean":
		vapp.SetNativeAppearance(vapp.AppearanceDark)
	case "paper", "mist", "clay", "ink", "aurora", "rose", "sunset", "lavender", "citrus":
		vapp.SetNativeAppearance(vapp.AppearanceLight)
	default:
		vapp.SetNativeAppearance(vapp.AppearanceUnknown)
	}
}
