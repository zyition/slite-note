# slite

极简桌面便签应用：单窗口承载 BlockNote 块级富文本编辑，本地持久化，窗口支持隐藏/唤起与全局快捷键。

## 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 桌面容器 | **Wails v3** (v3.0.0-beta.9) | 见 [ADR-0001](./docs/adr/0001-wails-v3-instead-of-neutralino.md)：req.md 原定 Neutralino 不支持全局快捷键（核心需求），Wails v3 内置 GlobalShortcut + SystemTray，Windows 构建纯 Go 零 CGO |
| 前端 | Vite 8 + React 19 + TypeScript + Tailwind v4 | CSS-first 配置，无 tailwind.config.js |
| 编辑器 | BlockNote 0.54 (`@blocknote/mantine`) | 斜杠菜单 / 拖拽手柄 / 气泡工具栏 / Markdown 快捷键 |
| 图标 | lucide-react | |

## 功能

- **窗口**：无边框自绘标题栏（40px，中部拖拽区）、默认 360 宽、首次启动左贴边全高、可自由调整尺寸
- **图钉**：置顶切换，跨重启持久化（settings.json）
- **3 主题**：便签黄 / 极简灰 / 深色，顶栏循环切换，持久化
- **全局热键**：`Alt+Shift+S` 切换窗口显隐（应用无需聚焦，核心功能）
- **关闭 = 隐藏**：应用驻留后台，托盘（显示/隐藏、退出）+ 热键唤回
- **编辑器**：BlockNote 全套（`/` 斜杠菜单、待办 Check List、代码块、引用等）
- **自动保存**：编辑防抖 800ms 写入本地
- **多便签**：顶栏下拉切换、新建
- **标题**：默认取首行文本截断；`Note.title` 字段预留手动覆盖（MVP 未做 UI）
- **浏览器降级**：无 Wails 运行时自动用 localStorage + no-op 窗口控制，可直接 `pnpm dev` 调试 UI

## 数据位置

- 便签：`%APPDATA%\slite\notes.json`（单文件，`Store` 接口抽象，未来可换 vault 形态，见 [ADR-0002](./docs/adr/0002-mvp-single-file-store-behind-interface.md)）
- 设置：`%APPDATA%\slite\settings.json`（主题、置顶）

## 开发

工具链：Windows 侧 mise 管理（node / pnpm / go），Go 模块走 goproxy.cn，npm 走 npmmirror（见 `frontend/.npmrc`）。

```bash
# 安装 wails3 CLI（写入 %USERPROFILE%\.local\bin）
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.9

# 前端依赖
cd frontend && pnpm install

# 桌面运行（wails dev，Vite HMR）
wails3 build          # 产物 bin\slite.exe

# 纯浏览器调试 UI（降级模式）
cd frontend && pnpm dev   # http://localhost:9245
```

> 构建脚本要求 `PACKAGE_MANAGER=pnpm` 且 PATH 含 mise shims 与 wails3 所在目录：
> `$env:PATH = "$env:LOCALAPPDATA\mise\shims;$env:USERPROFILE\.local\bin;" + $env:PATH; $env:PACKAGE_MANAGER = 'pnpm'; wails3 build`

## 结构

```
├── main.go            # Wails 应用：窗口/托盘/热键/事件
├── store.go           # Store 服务（bindings）：笔记与设置持久化
├── icons/             # 托盘图标（genicon 生成）
├── tools/genicon/     # 图标生成器（Go，image/png 绘制）
├── frontend/
│   ├── src/
│   │   ├── App.tsx            # 状态编排：boot/保存/主题/新建/切换
│   │   ├── components/        # TitleBar / NotePicker / Editor
│   │   ├── services/          # bridge（适配器）/ i18n / theme / title
│   │   └── types/note.ts      # Note / Settings 领域类型
│   └── bindings/             # wails3 生成的 TS bindings（勿手改）
└── docs/adr/          # 决策记录
```

## 路线图（MVP 之后的推迟项）

- 删除便签、标题手动覆盖 UI
- 尺寸/位置记忆（跨重启记住窗口大小位置）
- vault 存储形态（可配置数据目录、每便签一文件）
- 设置页（托盘菜单入口）
- i18n 框架（文案已集中 `src/services/i18n.ts`，英文优先）
- 退出入口加强（设置页内）
