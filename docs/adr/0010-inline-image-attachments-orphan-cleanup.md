# 内联图片附件 + 孤儿附件回收策略

ADR-0004 已定义「每便签一文件 + 附件目录 + AssetServer 相对引用」的存储形态，但附件管线（`SaveAttachment` / AssetServer / `uploadFile`）与孤儿回收一直未落地——schema 当时也把 image/音视频块排除在外（`Editor.tsx` 注释 "no attachment pipeline yet"）。本 ADR 落地 ADR-0004 的图片路径，并明确孤儿附件回收的产品与工程策略。

## 动机

- 便签场景高频来自「截图粘贴」，需要一个能本地内联图片的路径。
- 图片二进制不能内嵌 JSON：膨胀 33%、写放大、云同步负担、视频不可流式——沿用 ADR-0004 的拒绝理由（base64）。
- BlockNote（0.54）没有内置资源回收，无 `deleteFile` 回调、无引用计数（官方确认）；且 undo（`Ctrl+Z`）会重新插回被删的 block，若「删除即回收」会让撤销后图片永久裂图。官方推荐的做法是「在存储层比较当前文档状态」而非「删除事件即时回收」。

## 决策

1. **粘贴/上传图片走附件文件**：`uploadFile` → Go `SaveAttachment` 把字节按内容哈希写为 `attachments/<sha256[:16]>.ext`（哈希去重，跨 note 共享同一文件），返回相对引用 `attachments/<hash>.<ext>`；该引用存入块 `props.url`。渲染时编辑器 `resolveFileUrl` 把相对引用转成 `/attachments/<hash>.<ext>`，由 AssetServer 中间件从数据目录 `http.ServeFile`（`path.Base` 防路径穿越）。**不用 base64、不用 `file://`**。
   - **屏蔽「嵌入链接」入口**：BlockNote 0.54 无配置项单独关闭 embed tab（GitHub TypeCellOS/BlockNote#1613 为此提的 feature request，尚未实现），故禁用整块 FilePanel（`filePanel:false`）。本地图片通过粘贴/拖拽（走 `uploadFile`）插入，不受影响。

2. **孤儿附件回收 = 差集扫描**：`attachments/` 目录文件集 − 所有 Note blocks 中 `props.url`/`props.src` 以 `attachments/` 开头的引用集。全库扫描（跨 note），共享图在任一 Note 仍引用时不删。
   - **回收时机**：只在启动时。重启后 undo 栈已清空、保存的 blocks 即文档真相，扫描结果可靠；规避会话内 undo 恢复窗口。
   - **只删 `attachments/` 内文件**：差集以目录文件为基准，天然不触达 datadir 之外（外部 URL/绝对路径引用不参与）。
   - **删除失败静默跳过**并记 `debugLog`（权限/同步进程占用不中断其余）。
   - **单文件删除失败不阻断**整体清理。

3. **产品策略**（收敛自产品决策）：设置面板新增 **Attachments** section ——「**Clean now**」手动清理按钮 +「**启动时自动清理**」开关（**默认关**）。手动清理前弹确认，提示「undo 无法恢复已删除的文件」（清理发生在磁盘层，undo 只能恢复编辑器里的块）。

## Considered Options

- **base64 内嵌**：单文件自包含、零附件管线；但膨胀 33%、写放大、云同步累、不可流式。ADR-0004 已否决，此处沿用，不再展开。
- **删除即回收**（由 `onChange` 的 `delete` change 立刻删文件）：undo/redo 重插块会裂图；引用计数易被 undo/redo/move/paste 破坏而误删或漏删。不选。
- **启动差集扫描 + 默认关 + 手动清理**：吸收 joplin-vacuum 的教训（Joplin 官方因无法保证安全而不自动删；joplin-vacuum 用 dry-run、试删一、手动确认），把「自动」做成显式开关，手工触发始终可回溯。选此。

## Consequences

- `store.go`：新增 `SaveAttachment`、`CleanOrphanAttachments`、`attachmentExt`、孤儿扫描；`LoadNotes` 拆出无锁 `loadNotesLocked` 供扫描复用；`copyDataDirContents` 与 `validateDataDir` 纳入 `attachments`（datadir move/set 不丢附件目录）。对应单元测试。
- `main.go`：`AssetOptions.Middleware` 拦截 `/attachments/*`；启动时按 `autoCleanAttachments` 开关后台清理。
- 前端：`Editor.tsx` 恢复 `image` 块并配 `uploadFile`/`resolveFileUrl`；`bridge.ts` 增 `uploadAttachment`/`resolveAttachmentUrl`/`cleanOrphanAttachments`；`SettingsPanel` 增 Attachments section。
- 浏览器 fallback：`uploadAttachment` 退化为 data URL（仅调试用，不持久化）。
- 目前仅启用图片；video/audio/file 块仍排除——它们共用同一附件管线，启用只需在 schema 加回对应 `blockSpec`。
