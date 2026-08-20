# macOS 透明度语义：背景 alpha（内容保持清晰）

## 背景

Windows 的窗口透明度经 `WS_EX_LAYERED` 实现**整体 alpha**（文字、内容一起淡出）。Wails v3 没有跨平台的 `SetOpacity` 运行时 API；macOS 上的官方透明路径是：

- 创建时 `MacWindow.Backdrop`（`Transparent` / `Translucent` 毛玻璃 / `LiquidGlass`，macOS 15+），Backdrop **运行时不可切换**；
- 运行时 `SetBackgroundColour(R,G,B,A)` —— macOS 上 A 通道真实生效（`NSWindow.backgroundColor` 带 alpha），Windows 上 A 被忽略（0/255 之外无效）。

无真机约束（ADR-0006 背景）下，盲写 `NSWindow.alphaValue`（cgo）行为不可验证，且整体 alpha 有"窗口全透明不可见"的灾难风险。

## 决策

macOS 的"透明度"= **背景 alpha**，与 Windows 的"整体 alpha"并列但语义不同：

- 窗口创建时 `Backdrop = MacBackdropTransparent`（窗口壳与 WebView 背景透明，透出桌面）。
- 前端 CSS：`body` / `.titlebar` 背景经 `color-mix(… var(--bg-opacity, 100%), transparent)` 混入透明度；`--bg-opacity` 由 App.tsx 依据 `settings.opacity` 设置（macOS 生效，其他平台恒为 100% = 不透明）。
- 运行时 `SetBackgroundColour(themeRGB, alpha)` 同步窗口背景色与 CSS（bridge.setWindowBackground 新增 alpha 参数，Windows 传 255 不受影响）。
- 设置面板滑块保留（50–100%），macOS 上效果为"便签纸背景透出桌面、文字清晰"。
- 模态浮层（设置/主题选择/快捷键面板）打开时前端强制 `--bg-opacity = 100%`，对应 Windows 的 `setOpacityOverride`。

## Consequences

- **无 cgo、无原生盲写**：视觉完全由 CSS 控制，不会出现"窗口消失"类灾难（内容永不变透明）。
- 同名设置在不同平台视觉不同（Win 整体淡出 / mac 背景透出）——文档与 CONTEXT.md 明确标注"平台语义不同"。
- 毛玻璃（`Translucent` / `LiquidGlass`）列为后续增强：需要真机/用户反馈调配色（黄/灰/暗三主题 × 毛玻璃），且运行时不可切换（改设置需重启）。
- Linux 透明受限（GNOME/Mutter 剥 alpha），后续适配按本 ADR 的"创建时透明 + CSS alpha"思路降级处理。
