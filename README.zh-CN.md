# slite-note

极简桌面便签应用：单窗口承载 BlockNote 块级富文本编辑，本地持久化，窗口支持隐藏/唤起与全局快捷键。随开随记、不打扰。

> 本应用与 [slite.com](https://slite.com) 无关。

[English](./README.md)

## 功能

- **全局热键**（默认 `Alt+Shift+S`，可在设置中改）任意场景唤起/隐藏窗口，无需聚焦
- **关闭 = 隐藏**：驻留系统托盘，从托盘菜单退出
- **置顶图钉**、**窗口透明度**（50–100%）、**主题跟随系统深/浅色**（或固定便签黄）
- **窗口位置记忆**：大小与位置跨重启保留；显示器拔出时自动回退默认位置
- **静默自启动**：登录时不弹窗，热键唤出
- **块级编辑器**：BlockNote —— `/` 斜杠菜单、待办清单、标题、引用、代码块、拖拽排序、气泡工具栏
- **Markdown 导入/导出**：便签下拉单条导出/导入 `.md`，设置页一键导出全部
- **自动保存**（800ms 防抖）到本地单文件
- **多便签**：顶栏下拉切换/新建，便签菜单删除
- **数据可迁移**：设置页一键更换数据目录
- **零遥测**：全部数据留在本机

## 安装

- **Windows 10/11**，需 [WebView2 运行时]（多数系统已内置；安装包可在旧机器上自动补装）。

### 渠道

| 渠道 | 方式 |
|---|---|
| GitHub Releases | `slite-note-setup.exe`（NSIS 安装包）或便携版 `slite-note-<ver>-windows-amd64.zip` |
| 源码构建 | 见下方 |

> 便携版需系统已装 WebView2；安装包在旧机器上会自动安装。

## 构建

工具链：Go 1.25+、Node/pnpm、[Wails v3 CLI]。

```bash
cd frontend && pnpm install     # 前端依赖
wails3 build                    # 产物 bin\slite-note.exe
```

> Windows 下需 `PACKAGE_MANAGER=pnpm`，且 PATH 含 mise shims 与 wails3 所在目录。

### 纯浏览器调试 UI

无 Wails 运行时自动降级为 localStorage + 空窗口控制，可直接在浏览器调 UI：

```bash
cd frontend && pnpm dev         # http://localhost:9245
```

## 数据位置

| 内容 | 位置 |
|---|---|
| 便签 | `%APPDATA%\slite\notes.json`（单文件，设置页 *Change location…* 可迁移） |
| 设置 | `%APPDATA%\slite\settings.json` |
| WebView2 数据 | `%LOCALAPPDATA%\slite\webview` |
| 日志 | `%APPDATA%\slite\log.txt`（仅调试） |

## 隐私

完全离线运行：无遥测、无统计、运行时零网络请求，便签数据不出本机。

## 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 桌面容器 | **Wails v3** | 见 [ADR-0001](./docs/adr/0001-wails-v3-instead-of-neutralino.md) |
| 前端 | Vite + React 19 + TypeScript + Tailwind v4 | CSS-first，无 tailwind.config.js |
| 编辑器 | BlockNote 0.54 | `/` 菜单、拖拽把手、气泡工具栏 |
| 图标 | lucide-react | |

## 参与贡献

见 [CONTRIBUTING.md](./CONTRIBUTING.md) 与 [AGENTS.md](./AGENTS.md)。

## 致谢

基于 [Wails]、[BlockNote]、[React]、[Tailwind CSS] 与 [lucide] 构建。
完整第三方声明见 [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)。

## 许可

[MIT](./LICENSE) © 2026 zyition

[WebView2 运行时]: https://developer.microsoft.com/en-us/microsoft-edge/webview2/
[Wails v3 CLI]: https://wails.io
[Wails]: https://wails.io
[BlockNote]: https://www.blocknotejs.org
[React]: https://react.dev
[Tailwind CSS]: https://tailwindcss.com
[lucide]: https://lucide.dev
