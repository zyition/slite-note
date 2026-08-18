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
Note 的显示名。默认取首行文本截断；手动覆盖为后续迭代能力。
_Avoid_: 标题字段, heading

**Theme**:
全局（非每便签）的配色方案：便签黄 / 极简灰 / 深色。
_Avoid_: skin, mode

**Always on Top**:
窗口置顶状态，由顶栏图钉切换，跨重启持久化。
_Avoid_: pin（作为名词）, 钉住

**Global Hotkey**:
系统级快捷键 `Alt+Shift+S`，切换窗口显示/隐藏，应用无需聚焦。
_Avoid_: shortcut（作为系统级概念）, 快捷键

**Tray**:
系统托盘图标及菜单（显示/隐藏、退出）。关闭按钮隐藏窗口后，Tray 是退出应用的唯一入口。
_Avoid_: 托盘图标（仅指图标）, notification area

**Hide / Summon**:
关闭按钮将窗口隐藏（应用驻留后台）；Global Hotkey 或 Tray 唤起窗口。
_Avoid_: minimize（与系统最小化混淆）, 最小化

**Auto-save**:
编辑内容防抖 800ms 后写入本地存储的机制。

**Store**:
Note 数据持久化抽象层（接口），MVP 为单文件实现，未来可替换为 Vault 实现。
_Avoid_: storage（与浏览器 localStorage 混淆）, 存储

**Vault**:
（未来能力）可配置数据目录、每 Note 一个文件的存储形态，类似 Obsidian vault。
_Avoid_: 仓库, data dir（作为领域词）

**Settings**:
应用级偏好持久化（当前含 Theme 与 Always on Top 状态），存于 settings.json。
_Avoid_: config, 设置页（页面为 UI 概念，后续迭代）
