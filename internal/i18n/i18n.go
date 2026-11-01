// Package i18n contains Peck's user-facing language catalogs.
package i18n

import "strings"

// Language identifies a supported catalog.
type Language uint8

const (
	English Language = iota
	Chinese
)

func (l Language) String() string {
	if l == Chinese {
		return "zh-CN"
	}
	return "en"
}

// FromTag maps a BCP-47 language tag to the first supported catalog.
func FromTag(tag string) (Language, bool) {
	primary, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	switch primary {
	case "zh":
		return Chinese, true
	case "en":
		return English, true
	default:
		return English, false
	}
}

// FromPreferredLanguages chooses the first supported system language.
func FromPreferredLanguages(tags []string) Language {
	for _, tag := range tags {
		if l, ok := FromTag(tag); ok {
			return l
		}
	}
	return English
}

// Messages is the complete application catalog. Values are intentionally
// grouped by screen so call sites remain readable and adding a language does
// not require changing the UI state model.
type Messages struct {
	language                                                                                                     Language
	AppOnlyMacOS, StoppingQuit, TrayTooltip                                                                      string
	MenuOpen, MenuChoose, MenuSettings, MenuQuit                                                                 string
	InspectFailed, InspectCanceled, LinkNoMatch, LinkMultiple                                                    string
	OpenFailed, InvalidLink, OneApp, InvalidApp, QueueFull, QueueRetained                                        string
	ChooseDialog, ProgressValidate, LaunchRequest                                                                string
	SettingsTitle, SettingsDescription, Language, English, Chinese                                               string
	Theme, Motion, RequireTrust, Shortcut, ShortcutUnavailable, Quit, Back                                       string
	MainSettings, MainHide                                                                                       string
	IdleName, IdleSubtitle, Inspecting, InspectFailedSubtitle, VersionMissing                                    string
	LocalApp, LinkProject, PrepareTitle, PrepareMessage, ResolveTitle, ResolveMessage                            string
	ProfileTitle, ProfileMessage, ProcessingTitle, Stopping, Signature, Quarantine                               string
	ProgressSign, ProgressQuarantine, ProgressFinish                                                             string
	QuarantineNone, QuarantineRemoved, QuarantineAlready                                                         string
	Completed, Canceled, Failed, Recheck, Opening, OpenSubmitted                                                 string
	ResultDoneMessage, ResultChangedFmt, ResultCanceledMessage                                                   string
	TrustCheckbox, Choose, Stop, StopChecking, Processing, Replace, Confirm, ConfirmOpen                         string
	Finish, Open, OpeningShort, Again                                                                            string
	SignatureUnsigned, SignatureAdhoc, SignatureAdhocInvalid, SignatureValid, SignatureInvalid, SignatureUnknown string
	StatusSignatureAdhoc, StatusSignatureValid                                                                   string
	ThemeLabels                                                                                                  map[string]string
	ThemeDescriptions                                                                                            map[string]string
}

var EnglishMessages = Messages{
	AppOnlyMacOS: "Peck currently supports macOS only", StoppingQuit: "Stopping the current step before quitting…", TrayTooltip: "Peck · Process apps you trust",
	MenuOpen: "Open Peck", MenuChoose: "Choose app…", MenuSettings: "Settings…", MenuQuit: "Quit Peck",
	InspectFailed: "Inspection incomplete: %s", InspectCanceled: "Inspection stopped. Choose an app to try again.", LinkNoMatch: "No matching local app was found. Choose an app manually.", LinkMultiple: "Multiple matching apps were found. Choose the target manually.", OpenFailed: "Open request incomplete: %s", InvalidLink: "This Peck link cannot be used: %s", OneApp: "Choose one .app application at a time.", InvalidApp: "Choose a .app application. Extract archives and install disk images first.", QueueFull: "The pending request queue is full. Finish the current app first.", QueueRetained: "The new request was kept; the current app was not replaced.", ChooseDialog: "Choose an app to process", ProgressValidate: "Rechecking the application…", LaunchRequest: "Requesting app open",
	SettingsTitle: "Peck · Settings", SettingsDescription: "Make Peck work the way you prefer", Language: "Language", English: "English", Chinese: "简体中文", Theme: "Theme", Motion: "Play woodpecker animation (walk, peck, and eat)", RequireTrust: "Ask me to confirm the app source before processing", Shortcut: "Global shortcut: ⇧⌘P · In-window: ⌘W / ⌘, / ⌘O / ⌘Q · Reduce Motion takes priority.", ShortcutUnavailable: "Global shortcut unavailable; use the menu bar · In-window: ⌘W / ⌘, / ⌘O / ⌘Q · Reduce Motion takes priority.", Quit: "Quit Peck", Back: "Back", MainSettings: "Settings", MainHide: "Hide",
	IdleName: "Give the blocked app to Peck", IdleSubtitle: "Drop a .app to inspect it, then confirm each step", Inspecting: "Inspecting…", InspectFailedSubtitle: "Inspection incomplete; choose again", VersionMissing: "Version unavailable", LocalApp: "Local app: %s", LinkProject: "From project link: %s", PrepareTitle: "Workspace ready", PrepareMessage: "Drop a .app from Finder. Peck inspects it before asking you to confirm.", ResolveTitle: "Finding the local app", ResolveMessage: "A matching app will be inspected automatically.", ProfileTitle: "Building the app profile", ProfileMessage: "Reading the signature, quarantine attributes, and identity; no files are changed.", ProcessingTitle: "Processing the confirmed app", Stopping: "Stopping; waiting for the current step…", Signature: "Signature", Quarantine: "Quarantine", QuarantineNone: "None", QuarantineRemoved: "Found %d item(s); remove during processing", QuarantineAlready: "None (already processed)", Completed: "Processing complete", Canceled: "Processing stopped", Failed: "Processing incomplete", Recheck: "Recheck required", Opening: "Requesting app open", OpenSubmitted: "Open request submitted", ResultDoneMessage: "The check passed; you can try opening the app.", ResultChangedFmt: "Removed %d quarantine item(s); you can try opening the app.", ResultCanceledMessage: "Recheck to continue. Completed changes were kept.", TrustCheckbox: "I trust this app's source and approve this change", Choose: "Choose app…", Stop: "Stop", StopChecking: "Stopping…", Processing: "Processing…", Replace: "Replace app…", Confirm: "Confirm and process", ConfirmOpen: "Confirm and prepare to open", Finish: "Done", Open: "Open app", OpeningShort: "Requesting…", Again: "Open again", SignatureUnsigned: "Unsigned", SignatureAdhoc: "Existing local ad-hoc signature", SignatureAdhocInvalid: "Incomplete local signature; quarantine will be removed during processing", SignatureValid: "Signature verified", SignatureInvalid: "Signature verification failed", SignatureUnknown: "Signature status unknown", StatusSignatureAdhoc: "Existing local signature; will keep", StatusSignatureValid: "Verified; will keep", ProgressSign: "Creating a local signature", ProgressQuarantine: "Removing the confirmed quarantine attributes", ProgressFinish: "Verifying the result",
}

var ChineseMessages = Messages{
	AppOnlyMacOS: "Peck 当前仅支持 macOS", StoppingQuit: "正在停止，完成当前步骤后退出。", TrayTooltip: "Peck · 处理你信任的应用", MenuOpen: "打开 Peck", MenuChoose: "选择应用…", MenuSettings: "设置…", MenuQuit: "退出 Peck",
	InspectFailed: "检查未完成：%s", InspectCanceled: "已停止检查，可重新选择应用。", LinkNoMatch: "未找到匹配的本地应用，请点击“选择应用…”手动选择。", LinkMultiple: "找到多个匹配应用，请点击“选择应用…”确认目标。", OpenFailed: "打开请求未完成：%s", InvalidLink: "这个 Peck 链接无法使用：%s", OneApp: "请一次选择一个 .app 应用。", InvalidApp: "请选择 .app 应用。压缩包和安装镜像请先解压或安装。", QueueFull: "待处理请求已满，请先完成当前应用。", QueueRetained: "新请求已保留，当前应用不会被替换。", ChooseDialog: "选择要处理的应用", ProgressValidate: "正在重新核对应用…", LaunchRequest: "正在请求打开",
	SettingsTitle: "Peck · 设置", SettingsDescription: "让 Peck 按你的习惯工作", Language: "语言", English: "English", Chinese: "简体中文", Theme: "主题", Motion: "播放啄木鸟动画（走动、啄门与吃虫）", RequireTrust: "处理前要求我确认应用来源", Shortcut: "全局快捷键：⇧⌘P  ·  窗口内：⌘W / ⌘, / ⌘O / ⌘Q  ·  减少动态效果优先。", ShortcutUnavailable: "全局快捷键不可用，可从菜单栏唤起  ·  窗口内：⌘W / ⌘, / ⌘O / ⌘Q  ·  减少动态效果优先。", Quit: "退出 Peck", Back: "返回", MainSettings: "设置", MainHide: "收起",
	IdleName: "把受阻的应用交给 Peck", IdleSubtitle: "拖入 .app，先检查，再由你确认", Inspecting: "正在检查…", InspectFailedSubtitle: "检查未完成，可重新选择", VersionMissing: "版本未提供", LocalApp: "本机应用：%s", LinkProject: "来自项目链接：%s", PrepareTitle: "准备工作区", PrepareMessage: "从访达拖入 .app，Peck 会先检查，再由你确认处理。", ResolveTitle: "正在查找本机应用", ResolveMessage: "找到匹配应用后会自动检查，请稍候。", ProfileTitle: "正在建立应用画像", ProfileMessage: "读取签名、隔离属性与应用身份；此刻不会修改文件。", ProcessingTitle: "正在处理已确认的应用", Stopping: "正在停止，等待当前步骤结束…", Signature: "签名", Quarantine: "隔离标记", QuarantineNone: "未发现", QuarantineRemoved: "发现 %d 项，处理时移除", QuarantineAlready: "未发现（已处理）", Completed: "已完成本次处理", Canceled: "已停止处理", Failed: "本次处理未完成", Recheck: "需要重新检查应用", Opening: "正在请求打开应用", OpenSubmitted: "已提交打开请求", ResultDoneMessage: "检查已通过，可以尝试打开应用。", ResultChangedFmt: "已移除 %d 项隔离标记，可以尝试打开。", ResultCanceledMessage: "可重新检查后继续，已完成的修改会保留。", TrustCheckbox: "我信任这个应用的来源，并同意本次修改", Choose: "选择应用…", Stop: "停止", StopChecking: "正在停止…", Processing: "正在处理…", Replace: "更换应用…", Confirm: "确认并处理", ConfirmOpen: "确认并准备打开", Finish: "完成", Open: "打开应用", OpeningShort: "正在请求…", Again: "再次打开", SignatureUnsigned: "未签名", SignatureAdhoc: "已有本地 ad-hoc 签名", SignatureAdhocInvalid: "本地签名未封装完整，处理时移除隔离标记", SignatureValid: "签名校验通过", SignatureInvalid: "签名校验未通过", SignatureUnknown: "无法确定签名状态", StatusSignatureAdhoc: "已有本地签名，将保留", StatusSignatureValid: "校验通过，将保留", ProgressSign: "正在创建本地签名", ProgressQuarantine: "正在移除已确认的隔离标记", ProgressFinish: "正在核验处理结果",
}

func init() {
	EnglishMessages.language = English
	ChineseMessages.language = Chinese
	labels := map[string]string{"system": "跟随系统", "rose": "玫瑰云", "sunset": "晚霞", "paper": "暖纸", "midnight": "夜林", "mist": "雾蓝", "clay": "陶土", "ink": "墨金", "aurora": "极光", "ocean": "深海", "lavender": "薰衣草", "citrus": "柑橘"}
	desc := map[string]string{"system": "自动匹配 macOS 的浅色或深色", "rose": "玫瑰、薰衣草与柔光", "sunset": "蜜桃、珊瑚与莓果余晖", "paper": "奶油纸张与青绿光泽", "midnight": "夜幕、薄荷与深林蓝绿", "mist": "晨雾蓝与柔和钴色", "clay": "日落陶土与莓果光晕", "ink": "象牙纸、墨色与琥珀金", "aurora": "冰川青、薄荷与暮蓝", "ocean": "深海蓝与清透蓝光", "lavender": "暮紫、兰花与莓紫光", "citrus": "柠檬、蜜糖与橘红"}
	EnglishMessages.ThemeLabels = map[string]string{"system": "Follow System", "rose": "Rose Cloud", "sunset": "Sunset", "paper": "Warm Paper", "midnight": "Night Forest", "mist": "Blue Mist", "clay": "Clay", "ink": "Ink & Gold", "aurora": "Aurora", "ocean": "Deep Ocean", "lavender": "Lavender", "citrus": "Citrus"}
	EnglishMessages.ThemeDescriptions = map[string]string{"system": "Match macOS light or dark appearance automatically", "rose": "Rose, lavender, and soft light", "sunset": "Peach, coral, and berry afterglow", "paper": "Cream paper and green glow", "midnight": "Night, mint, and forest teal", "mist": "Morning blue mist and cobalt", "clay": "Sunset clay and berry halo", "ink": "Ivory paper, ink, and amber gold", "aurora": "Glacier cyan, mint, and dusk blue", "ocean": "Deep blue and clear ocean light", "lavender": "Dusk purple, orchid, and berry violet", "citrus": "Lemon, honey, and orange red"}
	ChineseMessages.ThemeLabels, ChineseMessages.ThemeDescriptions = labels, desc
}

func For(l Language) Messages {
	if l == Chinese {
		return ChineseMessages
	}
	return EnglishMessages
}

// CoreText translates the stable diagnostics emitted by the safety-critical
// core package while preserving paths, tool output, and other user supplied
// detail. Core is intentionally kept free of UI catalog dependencies.
func (m Messages) CoreText(text string) string {
	if m.LanguageIsChinese() || text == "" {
		return text
	}
	replacements := []struct{ from, to string }{
		{"系统评估服务暂时异常：macOS 未返回可确认的结果，请稍后重试。Peck 未修改应用。", "The system assessment service is temporarily unavailable: macOS did not return a confirmable result. Try again later. Peck did not modify the app."},
		{"系统评估暂不可用", "System assessment unavailable"},
		{"应用未通过 macOS 安全评估：请从开发者处确认应用来源和签名。Peck 未修改应用。", "The app did not pass macOS security assessment. Confirm its source and signature with the developer. Peck did not modify the app."},
		{"应用未通过安全评估", "The app did not pass security assessment"},
		{"无法确认应用签名状态，请从开发者处取得完整应用", "The app signature could not be verified. Obtain a complete app from the developer"},
		{"签名校验未通过，停止处理；请从项目来源重新下载", "Signature verification failed; processing stopped. Download the app again from the project source"},
		{"应用位于只读磁盘或安装镜像，请先将 .app 复制到下载文件夹或应用程序文件夹，再重新选择", "The app is on a read-only disk or installer image. Copy the .app to Downloads or Applications, then choose it again"},
		{"应用缺少 Contents/Info.plist，请从项目来源重新下载", "The app is missing Contents/Info.plist. Download it again from the project source"},
		{"无法读取应用信息文件", "The app information file could not be read"},
		{"应用信息文件格式无效", "The app information file has an invalid format"},
		{"应用缺少可执行主程序", "The app is missing its executable"},
		{"应用信息包含不支持的控制字符", "The app information contains unsupported control characters"},
		{"应用路径无效", "The app path is invalid"},
		{"链接目标不是 .app 应用", "The link target is not a .app application"},
		{"不能处理系统目录中的应用", "Apps in system directories cannot be processed"},
		{"包内符号链接路径无效", "The bundle symlink path is invalid"},
		{"应用在检查时发生变化，请重新选择", "The app changed during inspection. Choose it again"},
		{"不处理具有多个硬链接的文件", "Files with multiple hard links are not processed"},
		{"应用文件数量超过本版本的检查范围", "The app has more files than this version can inspect"},
		{"应用包含其他卷的内容，无法处理", "The app contains content from another volume and cannot be processed"},
		{"应用超过本版本的 16 GB 检查范围", "The app exceeds this version's 16 GB inspection limit"},
		{"应用包含不支持的特殊文件", "The app contains unsupported special files"},
		{"只支持普通文件和目录，不处理硬链接或特殊文件", "Only regular files and directories are supported; hard links and special files are not processed"},
		{"应用在检查后发生变化，请重新选择并确认", "The app changed after inspection. Choose it again and confirm"},
		{"隔离标记移除后的验证未通过", "Verification after removing quarantine attributes failed"},
		{"仅可打开已通过本次处理核验的应用", "Only an app verified by this processing run can be opened"},
		{"应用最终核验未通过", "Final app verification failed"},
		{"检查结果已过期，请重新选择应用", "The inspection has expired. Choose the app again"},
		{"请先选择并检查应用", "Choose and inspect an app first"},
	}
	for _, replacement := range replacements {
		text = strings.ReplaceAll(text, replacement.from, replacement.to)
	}
	return text
}

func (m Messages) LanguageIsChinese() bool { return m.language == Chinese }
