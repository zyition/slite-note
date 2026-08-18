# Role & Goal

你是一名精通现代前端工程化与轻量级桌面端开发的资深全栈工程师。

请为我开发一个跨平台的**极简桌面便签应用（Sticky Notes）**，要求前端具备**类飞书/Notion 的块级富文本编辑体验**，且无需任何 Rust/C++ 编译环境。



---



## 1. 技术栈规范

- **桌面容器**：Neutralino.js (neu CLI)

- **构建工具**：Vite (React + TypeScript)

- **UI & 样式**：Tailwind CSS + Lucide React (图标库)

- **核心编辑器**：BlockNote (`@blocknote/react` + `@blocknote/mantine`)

- **数据持久化**：Neutralino.js 本地文件系统 API (`Neutralino.filesystem`) 或内置 Storage



---



## 2. 核心功能与体验需求



### A. 桌面窗口特性（便签体验）

1. **无边框窗口 (Borderless)**：自定义顶部极简拖拽把手区（通过 CSS `-webkit-app-region: drag` 或 Neutralino 窗口拖拽 API 实现）。

2. **窗口置顶 (Always on Top)**：右上角提供“图钉”图标按钮，点击可切换窗口是否全局置顶。

3. **窗口控制**：右上角提供极简的“新建便签”、“最小化”和“关闭”按钮。

4. **自适应尺寸与轻量感**：默认窗口尺寸约为 `360px × 480px`，支持多主题底色切换（如便签黄、极简灰、深色模式）。



### B. 富文本编辑特性（类飞书云文档）

1. **BlockNote 集成**：

   - 支持 `/` 唤出斜杠菜单（标题、无序列表、待办事项 Checkbox、代码块、引用等）。

   - 支持块级拖拽手柄、选中文本弹出气泡工具栏（加粗、高亮、下划线）。

   - 支持 Markdown 快捷键输入（如 `# ` 自动变一级标题，`- ` 变列表）。

2. **自动保存 (Auto-save)**：

   - 监听编辑器内容变更，加入防抖机制（Debounce 500ms~1000ms）。

   - 自动将当前 Block JSON 数据写入本地指定路径文件（如 `./data/notes.json`）。

3. **多便签管理（极简设计）**：

   - 左侧或顶部提供折叠式简易便签列表，可快速切换不同的便签。



---



## 3. 架构与工程规范要求



### ① 配置文件要求

- 提供完整的 `neutralino.config.json`：

  - 配置 `modes.window`：设置 `borderless: true`、`width: 380`、`height: 500`、`minWidth: 300`、`minHeight: 300`。

  - 配置 `nativeBlockList` 或启用文件读写/窗口控制的相关权限白名单 (`filesystem.*`, `window.*`, `os.*`, `storage.*`)。

  - 配置开发模式代理到 Vite 本地开发服务器 (`http://localhost:5173`)。

- 提供 `vite.config.ts`：配置构建输出目录对应 Neutralino 的资源目录（如 `dist`）。



### ② 浏览器环境降级（Graceful Fallback）

- 在代码中封装一个 `storageAdapter` 和 `windowAdapter`：

  - 如果检测到 `window.Neutralino` 未加载（例如在纯浏览器调试页面时），自动降级使用浏览器的 `localStorage` 和 `console.log`，保证在纯浏览器模式下也能正常调试 UI 和编辑器。

  - 在 Neutralino 运行时中，正确调用 `Neutralino.init()` 并使用原生系统 API。



### ③ 项目结构建议

```text

├── src/

│   ├── components/

│   │   ├── TitleBar.tsx       # 自定义无边框顶栏（拖拽区、置顶、关闭）

│   │   ├── Editor.tsx         # BlockNote 编辑器核心封装

│   │   └── NoteSidebar.tsx    # 便签切换/新建列表（侧边栏或抽屉）

│   ├── services/

│   │   ├── neutralino.ts      # Neutralino SDK 初始化与适配层

│   │   └── storage.ts         # 读写本地 JSON 文件的防抖持久化逻辑

│   ├── types/

│   │   └── note.ts

│   ├── App.tsx

│   └── main.tsx

├── neutralino.config.json

├── package.json

├── vite.config.ts

└── tailwind.config.js
