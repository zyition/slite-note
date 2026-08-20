# Slite Context

Slite 是一个极简的桌面便签应用：单窗口承载 BlockNote 块级富文本编辑，便签数据本地持久化，窗口支持隐藏/唤起与全局快捷键。

## Language

**Note**:
一个独立的可编辑内容单元，由 BlockNote 块数组组成。
_Avoid_: Sticky, memo, 记事

**Note List**:
应用中全部 Note 的集合，通过顶部下拉（NotePicker）切换与新建。
_Avoid_: 便签列表, collection

**Title**:
Note 的显示名。默认取首行文本截断（deriveTitle）；可在 NotePicker 中手动重命名覆盖，覆盖名为空时回退到首行派生。
_Avoid_: 标题字段, heading

**Rename**:
在 NotePicker 行上点击铅笔进入内联编辑，Enter/失焦提交，Escape 取消；清空提交即清除覆盖。
_Avoid_: 重命名（作为动词短语）, title override（作为 UI 概念）

**Quick Switch**:
窗口内循环切换 Note 的快捷键（编辑器聚焦时同样生效）。Windows 为 `Ctrl+Tab` / `Ctrl+Shift+Tab`；macOS 为 `Cmd+Shift+[` / `Cmd+Shift+]`（避开系统级 Ctrl+Tab）。
_Avoid_: 标签页切换, tab switching

**Theme**:
全局（非每便签）的配色方案：跟随系统深/浅色（system），或固定便签黄（yellow）。
_Avoid_: skin, mode

**Always on Top**:
窗口置顶状态，由顶栏图钉切换，跨重启持久化。
_Avoid_: pin（作为名词）, 钉住

**Opacity**:
窗口透明度（50–100%），设置页滑块调节并持久化。平台语义不同：Windows 为整体 alpha（经 Win32 `WS_EX_LAYERED`），macOS 为背景 alpha（内容保持清晰，经 Backdrop + `SetBackgroundColour`）。
_Avoid_: alpha, 不透明度（作为设置项名）

**Global Hotkey**:
系统级快捷键（默认 `Alt+Shift+S`，可配置，跨平台一致；macOS 上 Alt=Option），切换窗口显示/隐藏，应用无需聚焦。
_Avoid_: shortcut（作为系统级概念）, 快捷键

**Tray**:
系统驻留图标及菜单（显示/隐藏、退出）。关闭按钮隐藏窗口后，Tray 是退出应用的唯一入口。Windows 为系统托盘图标，macOS 为菜单栏图标（单击弹菜单，遵循 mac 惯例）。
_Avoid_: 托盘图标（仅指图标）, notification area

**Hide / Summon**:
关闭按钮（macOS 亦含 `Cmd+W`）将窗口隐藏（应用驻留后台）；Global Hotkey 或 Tray 唤起窗口。
_Avoid_: minimize（与系统最小化混淆）, 最小化

**Silent Launch**:
以 `--silent` 参数启动（开机自启动场景）：窗口定位但不显示，由 Global Hotkey/Tray 唤起。
_Avoid_: 静默模式（与日志静默混淆）, background start

**Auto-save**:
编辑内容防抖 800ms 后写入本地存储的机制。

**Store**:
Note 数据持久化抽象层（接口），当前为单文件实现，未来可替换为 Vault 实现。
_Avoid_: storage（与浏览器 localStorage 混淆）, 存储

**Data Directory**:
Note 与 Settings 的存储目录（默认 Windows `%APPDATA%\slite` / macOS `~/Library/Application Support/slite`），可在设置页通过 "Change location…" 迁移。
_Avoid_: data dir（作为领域词，裸词）, 数据文件夹

**Vault**:
（未来能力）可配置数据目录、每 Note 一个文件的存储形态，类似 Obsidian vault。
_Avoid_: 仓库, data dir（作为领域词）

**Settings**:
应用级偏好持久化（Theme、Always on Top、Opacity、Global Hotkey、Silent Launch、Data Directory、Window Bounds），存于 settings.json。
_Avoid_: config, 设置页（页面为 UI 概念）

**Window Bounds**:
窗口的位置与尺寸（物理像素），随移动/缩放防抖持久化并跨重启恢复；目标屏幕不可见时回退默认定位。由 Go 侧独占写入。
_Avoid_: window position, 窗口布局
