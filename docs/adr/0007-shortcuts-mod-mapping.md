# 快捷键平台化：Mod 映射 + 冲突键单独决策

## 背景

前端快捷键硬编码 `Ctrl`（App.tsx、Editor.tsx、shortcuts.ts 的 cheatsheet），而 macOS 的主修饰键是 `Cmd`（⌘）。BlockNote 内置快捷键用 `Mod`（mac 上自动为 ⌘），无需改动；我们的自定义快捷键必须平台化，否则 mac 用户按 `Ctrl+B` 等组合完全无效、且 cheatsheet 显示错误。

另外两个键在 macOS 上是系统级占用，**不能** Mod 映射：

- `Ctrl+Tab`：系统"全键盘访问"的导航键（开启辅助功能时）。
- `Cmd+Tab`：系统 app 切换器（绝对禁用）。

## 决策

1. **Mod 抽象**：应用自定义快捷键统一为 `Mod` 前缀 —— macOS 用 `Cmd`、Windows/Linux 用 `Ctrl`，与 BlockNote 的 Mod 约定一致。映射表集中在 `shortcuts.ts`（`SHORTCUT_KEYS`）。
2. **冲突键单独决策**：
   - 切 Note（next/prev）：macOS = `Cmd+Shift+]` / `Cmd+Shift+[`（Chrome/Safari"切标签页"惯例，语义贴合 Note 切换）；Windows = `Ctrl+Tab` / `Ctrl+Shift+Tab`（不变）。
   - Redo：macOS = `Cmd+Shift+Z`（系统惯例）；Windows = `Ctrl+Y` / `Ctrl+Shift+Z`（不变）。
   - 全局热键默认值：macOS 保持 `Alt+Shift+S`（Alt=Option，物理键位一致、无系统占用，跨平台心智统一）。
3. **录制/显示平台化**（hotkey.ts）：修复 `metaKey → "Super"` 的语义错误 —— `Super` 是 Windows/Logo 键，mac 上必须映射为 `Cmd`（Wails 加速器语法）；mac 修饰符顺序 `Cmd→Ctrl→Alt→Shift`（⌘⌃⌥⇧）；cheatsheet 与设置面板在 mac 上以符号渲染（⌘⌥⇧⌃）。
4. **事件判断平台化**（App.tsx / Editor.tsx）：mac 分支以 `metaKey` 为主修饰键并排除 `ctrlKey`（反之亦然）；mac 切 Note 用 `e.code === "BracketLeft/Right"` 判断（`Shift+[` 的 key 在 US 布局是 `{`，code 更可靠）。

## Consequences

- 两平台常用快捷键语义对齐（新增 → Mod+N、设置 → Mod+,、主题循环 → Mod+Shift+T），肌肉记忆迁移成本低。
- cheatsheet / 设置面板按平台显示正确组合与符号；BlockNote 编辑区快捷键（Mod+B 等）天然一致。
- 未来新增快捷键默认走 Mod；再次遇到系统冲突键按"单独决策"流程处理（记录进本 ADR 或新增条目）。
- 全局热键（Alt+Shift+S）跨平台一致，注册器与设置面板零平台差异。
