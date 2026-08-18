# Wails v3 取代 Neutralino 作为桌面容器

req.md 指定的 Neutralino.js 无法提供全局快捷键（核心需求：Alt+Shift+S 唤起隐藏），其全部 16 个 API 命名空间均无全局热键能力，只能靠 PowerShell 伴生脚本 hack（窗口标题匹配、执行策略、仅 Windows，脆弱）。Wails v3 内置全局快捷键（Win32 RegisterHotKey）与系统托盘，Windows 构建为纯 Go、零 CGO、无 C 编译器依赖（已核查 v3 全部 Windows 源文件无 `import "C"`）；前端技术栈（Vite + React 19 + Tailwind v4 + BlockNote）与浏览器降级适配模式完全不变，仅容器层替换。Wails v3 为 beta（钉定 v3.0.0-beta.9），官方声明 desktop API 已稳定并有生产使用。

**Considered Options**
- Neutralino（req.md 原栈）：托盘/置顶/拖拽原生、二进制 ~2MB，但全局热键只能 PowerShell hack。
- Wails v2（stable）：无 CGO，但托盘无内置（getlantern/systray 需 CGO/gcc，违反"无 C 编译环境"约束），需手写 ~300 行 Win32 托盘。

**Consequences**
- 需要 Go 工具链（mise 管理，Windows 侧已装 1.26.6），构建流程从 `neu` 变为 `wails3`。
- 构建产物为 exe（WebView2 常驻系统），二进制体积大于 Neutralino。
- req.md 中 neutralino.config.json / nativeBlockList 等配置要求作废，由 Wails v3 窗口选项与 Go 侧权限模型替代。
