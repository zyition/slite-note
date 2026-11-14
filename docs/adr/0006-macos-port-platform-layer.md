# macOS 移植的平台抽象层（platform layer）

## 背景

main.go 内嵌了裸 Win32 调用：`SetWindowPos`（启动定位）、`GetWindowRect`（边界持久化）、`WS_EX_LAYERED + SetLayeredWindowAttributes`（透明度）、`KnownFolderPath`（Downloads）；store.go 里还有 HKCU Run 键（开机自启）、`ShellExecuteW`（打开 URL）、`explorer.exe`（打开数据目录）。这些能力在 macOS 上没有一一对应的 Wails 跨平台 API，必须按平台实现。

macOS 移植的两个硬约束：

1. **无真机**：darwin 构建只能在 CI mac runner 上编译（Wails mac 构建必须原生 cgo，不可交叉编译）。
2. **三平台预留**：Linux 后续适配（AGENTS.md backlog），抽象要按三平台形状设计，但只实现 Windows + macOS。

## 决策

同一 main 包内按 build tags 拆分平台文件，暴露统一函数签名（其余代码零平台分支）：

- `platform_windows.go`（`//go:build windows`）：现有 Win32 实现原样迁入 —— `setWindowOpacity` / `setOpacityOverride` / `applyWindowOpacity`、`setWindowBounds`、`saveWindowBoundsNow`、`userDownloadsDir`、空 `setupPlatformUI` / `registerPlatformHooks`。
- `platform_darwin.go`（`//go:build darwin`）：
  - 透明度 = no-op（视觉由前端 CSS 背景 alpha 承担，见 ADR-0008）；
  - `setWindowBounds` / `saveWindowBoundsNow` = Wails `SetPosition`/`SetSize`/`Position`/`Size`（逻辑点）× `Screen.ScaleFactor` 换算（Retina 下 Wails 的 screen 是物理像素、窗口 API 是逻辑点，必须转换）；
  - `setupPlatformUI` = 应用菜单栏（App/File/Edit/Window 角色 + Settings…/Cmd+,）；
  - `registerPlatformHooks` = `ApplicationWillTerminate`（flush 边界）+ `ApplicationShouldHandleReopen`（Dock 点击唤起隐藏窗口）。
- `store_platform_windows.go` / `store_platform_darwin.go`：`setLaunchAtStartup`/`getLaunchAtStartup`（Windows registry / macOS `app.Autostart` + `Arguments: ["--silent"]`）、`openURL`（ShellExecute / `open`）、`openDataDir`（explorer.exe / `open`）。

纯几何/字符串逻辑继续留在 `internal/windowutil`（无平台依赖，可单测）。

## Consequences

- **本地（Windows）只能验证 Windows 侧**；darwin 文件靠 CI mac runner 的 `go vet` + `go test` + 原生构建 + `--smoke` 启动冒烟兜底。
- 平台函数签名统一，未来 Linux 适配 = 新增 `platform_linux.go` + `store_platform_linux.go`，共享代码零改动。
- 前端平台分支（`isMac()`）集中在 `services/platform.ts` / `hotkey.ts` / `shortcuts.ts`，组件层不散落平台判断。
- Windows 行为零变化（纯搬迁，registry 键名、`slite` 旧键兼容、LWA 逻辑均保留）。

## 后续（Wails v3 正式版）

v3.0.0 / v3.1.0 转正后，其中三项能力有了官方跨平台 API，已收回共享层，`store_platform_*.go` 只剩自启 artifact 名：

- 开机自启：两侧统一 `app.Autostart`（`store_autostart.go`）——Windows 仍写 HKCU Run 的 `slite-note` 值，macOS 仍是 `com.zyition.slite-note`。旧实现按「值是否存在」判断，改名后指向死路径的 `slite` 旧键会被误报为已启用；官方实现比对注册命令与当前可执行文件路径，这类残留正确显示为未启用，因此不再需要清理旧键。
- 打开 URL：`app.Browser.OpenURL`。
- 打开数据目录：`app.Env.OpenFileManager(dir, false)`。

仍留在平台层：窗口透明度（Windows `WS_EX_LAYERED`，Wails 无公开 API）、启动定位（`SetWindowPos`，Wails 对隐藏窗口的 `SetPosition` 不可靠且会重置 alpha）、`userDownloadsDir`、系统深色偏好（`app.Env.IsDarkMode` 在 `application.New` 之前返回 false，而它服务于首帧背景色）。
