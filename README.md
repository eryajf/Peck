<p align="center">
  <img src="assets/peck-icon.svg" width="96" height="96" alt="Peck 图标">
</p>

<h1 align="center">Peck</h1>

<p align="center">一个住在 macOS 菜单栏里的应用打开助手</p>

<p align="center">
  <a href="https://github.com/eryajf/Peck">GitHub</a> ·
  <a href="https://github.com/vivaui/viva">Viva</a> ·
  <a href="https://linux.do">Linux DO</a>
</p>

Peck 基于 [Viva](https://github.com/vivaui/viva) 原生 UI 框架和 Go 构建，不使用 WebView 或前端框架。它帮助你检查一个 macOS 应用的来源、签名、隔离标记和包内结构，在你明确确认后，移除下载隔离标记、按需添加本地 ad-hoc 签名，并尝试打开应用。

Peck 不会替你判断一个应用是否安全，也不会修改系统级 Gatekeeper 设置。每一次处理都针对你选择的单个应用，并在执行前展示检查结果。


## 主要功能

- 拖入或选择 macOS 应用，查看来源、签名和隔离状态。
- 确认后移除隔离标记，或按需添加本地 ad-hoc 签名，并查看处理结果。
- 常驻菜单栏，支持中英文界面、主题切换和从项目页面唤起。

## 安装

从 [GitHub Releases](https://github.com/eryajf/Peck/releases) 下载适合你的 Mac 的版本并安装。

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

`project` 用于显示项目上下文，`bundle_id` 和可选的 `release` 只用于匹配本机候选应用。深链接不能传入本地路径、命令、凭据或自动执行权限；用户仍需核对真实路径并点击确认。

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

GitHub 的分支和标签会同步到 [CNB 仓库](https://cnb.cool/znb/Peck)，发布完成后也会同步 Release 和附件。请在 GitHub Actions Secrets 中配置具有 CNB 仓库写入权限的 `CNB_USERNAME` 和 `CNB_TOKEN`，并手动运行一次 “Sync code to CNB” 工作流以同步已有分支和标签；同步不会强制覆盖或删除 CNB 上的引用。

## 捐赠与支持

如果 Peck 对你有帮助，欢迎通过[捐赠支持页面](https://wiki.eryajf.net/reward/)请作者喝杯咖啡。你的反馈、问题报告和使用体验同样有帮助，欢迎在 [GitHub Issues](https://github.com/eryajf/Peck/issues) 中提出。

## 许可证

Peck 使用 [Apache License 2.0](LICENSE) 发布。
