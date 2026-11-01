<p align="center">
  <img src="assets/peck-icon-animated.svg" width="96" height="96" alt="Peck 动态图标">
</p>

<h1 align="center">Peck</h1>

<p align="center">一个住在 macOS 菜单栏里的应用打开助手</p>

<p align="center">
  <img src="assets/peck-badge-animated.svg" width="248" alt="交给 Peck 处理">
</p>

<p align="center">
  <a href="https://github.com/eryajf/Peck">GitHub</a> ·
  <a href="https://github.com/vivaui/viva">Viva</a>
</p>

Peck 基于 [Viva](https://github.com/vivaui/viva) 原生 UI 框架和 Go 构建，不使用 WebView 或前端框架。它帮助你检查一个 macOS 应用的来源、签名、隔离标记和包内结构，在你明确确认后，移除下载隔离标记、按需添加本地 ad-hoc 签名，并尝试打开应用。

Peck 不会替你判断一个应用是否安全，也不会修改系统级 Gatekeeper 设置。每一次处理都针对你选择的单个应用，并在执行前展示检查结果。


## 主要功能

- 从 Finder 拖入 `.app`，或通过文件选择器选择应用。
- 显示应用名称、版本、Bundle ID、完整路径、签名状态和隔离标记摘要。
- 在处理前明确展示将要执行的动作，默认不会覆盖已有有效签名。
- 只处理当前用户确认的目标应用，不扫描或批量修改已安装应用。
- 可移除 `com.apple.quarantine`，并在适用时选择添加本地 ad-hoc 签名。
- 处理结束后分别报告隔离、签名和启动结果。
- 通过 `peck://prepare` 深链接从开源项目页面直接进入准备流程。
- 菜单栏常驻、中文/English 界面、主题切换和减少动态效果支持。
- 原生啄木鸟动画：待机、啄击、应用图标飞出和完成反馈都使用 Viva Canvas 绘制。

## 安装

从 [GitHub Releases](https://github.com/eryajf/Peck/releases) 下载对应架构的 macOS ZIP，解压后将 `Peck.app` 放到 `/Applications` 或你自己的应用目录。

Release workflow 已接入 Developer ID 签名、公证和 stapling。配置完成后，从 GitHub Releases 下载的包可以按 macOS 的正常方式打开；如果系统提示目标应用不可写、位于只读磁盘映像或需要额外权限，请先把应用复制到本地可写目录，再重新选择。

## 使用

1. 启动 Peck，点击菜单栏里的啄木鸟图标打开面板。
2. 将需要处理的 `.app` 拖入面板，或点击“选择应用”。
3. 查看应用路径、签名状态、隔离标记和系统检查结果，确认它确实来自你信任的来源。
4. 选择要执行的处理项。移除隔离标记和本地 ad-hoc 签名是两个独立动作，未选择的动作不会执行。
5. 点击“确认并处理”，等待结果。处理完成后，只有点击“打开应用”才会尝试启动目标应用。

处理任务在后台执行；隐藏面板不会取消任务。“停止”只会取消尚未执行的后续步骤，并保留已经完成的结果。应用图标、名称和版本来自本机实际的 App bundle，不会因为深链接中的项目名称而自动信任某个文件。

### 从项目页面唤起 Peck

开源项目可以使用下面的链接把用户带入 Peck 的准备流程：

```text
peck://prepare?v=1&project=Example%20App&bundle_id=org.example.app
```

`project` 用于显示项目上下文，`bundle_id` 和可选的 `release` 只用于匹配本机候选应用。深链接不能传入本地路径、命令、凭据或自动执行权限；用户仍需核对真实路径并点击确认。页面接入可以直接使用 `assets/` 中的静态或动态 SVG 徽章。

## 动态 Logo 与接入素材

Peck 的公开接入素材位于 [`assets/`](assets/)：

- `peck-icon-animated.svg`：适合 README 标题或产品介绍。
- `peck-badge-animated.svg`：标准“交给 Peck 处理”徽章。
- `peck-badge-mini-animated.svg`：适合 README 徽章区。
- 同名不带 `-animated` 的文件：静态版本，适合不显示 SVG 动画的平台。

动画使用 SVG 内嵌 CSS，不依赖 JavaScript 或外部资源，并遵循 `prefers-reduced-motion`。这些素材可以直接用于项目 README、发布页和产品介绍。

## 开发与构建

需要 macOS 14 或更高版本，以及 Go 1.27.3 或更高版本。项目直接使用官方 `github.com/vivaui/viva` 模块：

```sh
go mod download
go run .
```

构建未签名的本地 App 和 ZIP：

```sh
make package
```

常用检查：

```sh
make version
make doctor
make test
make vet
make build
```

项目要求 Go 1.27.3 或更高版本。如果本机设置了 `GOTOOLCHAIN=local`，请先安装对应版本的 Go；否则 Go 无法自动切换到 `go.mod` 声明的工具链。

Peck 是使用 `viva/app.RunApplication` 的菜单栏应用，当前不提供 `viva dev` 热重载目标；Viva 的开发 worker 只适用于 `RunCanvas` 应用。请使用 `make run` 启动本地应用。

推送 `v*` 标签会触发 [Release Action](.github/workflows/release.yml)，在 Apple Silicon 和 Intel macOS runner 上生成经过签名、公证和 stapling 的 ZIP、DMG 及校验文件。发布前需要配置 Apple Developer 证书和 App Store Connect API key，缺少任一凭据时 workflow 会停止，不会上传未签名包。Apple Developer 证书、公证配置和 API key 通过 GitHub Actions Secrets 提供。

## 项目结构

| 目录 | 作用 |
| --- | --- |
| `internal/app/` | 菜单栏入口、窗口、状态管理、任务流程和设置 |
| `internal/core/` | App 检查、隔离标记处理、受限本地签名和结果复核 |
| `internal/brand/` | 啄木鸟绘制、动画时序、主题和运行时图像 |
| `internal/vui/` | 项目对 Viva UI 的小型适配层 |
| `assets/` | README 和项目页面使用的公开 SVG 素材 |

## 当前限制

- GitHub Release 的签名、公证和 stapling 依赖仓库 Secrets；本地使用 `--no-sign` 构建的包仍然不会签名。
- Peck 不能修复应用本身损坏、架构不匹配、缺少依赖或明确的恶意软件诊断。
- 本地 ad-hoc 签名不等同于 Developer ID 签名，也不代表 Apple 公证或安全认证。
- 普通用户权限无法处理的应用会明确提示原因，不会自动提权。
- 处理隔离标记不会证明目标应用安全；请始终结合下载来源、发布者和代码签名自行判断。

## 捐赠与支持

如果 Peck 对你有帮助，欢迎通过[捐赠支持页面](https://wiki.eryajf.net/reward/)请作者喝杯咖啡。你的反馈、问题报告和使用体验同样有帮助，欢迎在 [GitHub Issues](https://github.com/eryajf/Peck/issues) 中提出。

## 许可证

Peck 使用 [Apache License 2.0](LICENSE) 发布。
