# 每便签一文件存储 + 附件目录（存储架构演进）

ADR-0002 的单文件 `notes.json` 在 MVP 阶段够用，但三个真实约束下不可持续：

1. **数据不丢（OneDrive 场景）**：单文件整体损坏 = 全部便签丢失；数据目录若落在 OneDrive 同步范围，云同步进程与写入交错、多设备同时运行，存在整体损坏与冲突风险。SQLite 同样不适用——SQLite 官方明确警告网络/同步文件系统锁不可靠，"两客户端同时改同一文件导致损坏"，且 WAL 的 `-wal`/`-shm` 伴随文件会被 OneDrive 分别同步破坏一致性。
2. **性能**：编辑一条便签即全量重写整个文件（读写放大 O(N)）。
3. **多媒体落盘**：BlockNote 图片/音视频块只在 JSON 里存 URL 引用，当前未配置 `uploadFile`，本地媒体实际无法插入；未来支持时二进制必须与 JSON 分离存储（base64 内嵌会使 JSON 膨胀 33% 并放大云同步负担）。

决策：**每 Note 一个 JSON 文件 + 附件目录 + 目录即索引**，块内媒体以相对引用存储、经 Wails AssetServer 加载。动机是健壮性与性能，而非"类 Obsidian 的用户可手编"理念。

## 目标存储形态

```
<dataDir>/
├── settings.json            # 不变
├── notes/                   # 每便签一个文件，文件名 = note id + ".json"
│   ├── <id>.json            # 完整 Note（id/title/blocks/createdAt/updatedAt）
│   └── ...
└── attachments/             # 二进制附件，内容哈希命名，不可变
    └── <sha256[:16]>.<ext>
```

- **目录即索引**：无独立索引文件（无单点、无需一致性维护）。启动时扫描 `notes/*.json` 构建内存 map，几千条为毫秒级；运行期维护内存索引。数据量到几万条、扫描成为瓶颈时，再加 `index.json` 缓存或 SQLite 派生索引（文件仍是真相，索引可重建）——推迟到确有需要。
- **读写路径**：读 = 按 ID 直读单文件；写 = 仅重写被编辑的便签；删除 = 删文件。彻底消除全量读写放大。
- **损坏隔离**：单文件损坏只影响一条；沿用现有 corrupt 备份逻辑（备份坏文件后继续启动）。
- **OneDrive 友好**：同步冲突只产生单便签冲突副本，最坏丢一条的冲突版本且副本可恢复，绝不会全库损坏；附件哈希命名不可变，冲突/中断只影响单个文件。

## 多媒体：相对引用 + AssetServer

- 前端 `uploadFile(file)` → Go binding `Store.SaveAttachment(name, mime, bytes)` 写 `attachments/<hash>.<ext>`，返回相对引用 `attachments/<hash>.<ext>` 存入块 `props.src`（图片）/ `props.url`（音视频、文件）。
- 前端 `resolveFileUrl(url)`：`attachments/` 前缀 → 同源绝对路径 `/attachments/<hash>.<ext>`；远程 URL 原样返回。
- Go 侧 AssetServer 中间件拦截 `/attachments/*`，`http.ServeFile` 从数据目录读取（自带 Content-Type 与 Range 支持，视频播放必需），`filepath.Clean` 防路径穿越，仅放行该前缀。
- **不用 base64**（大文件内存与 JSON 膨胀、视频无法流式）；**不用 `file://`**（WebView2 安全策略禁用，Wails 官方确认）。
- 浏览器 fallback 模式退化为 data URL（仅调试用，不持久化二进制）。
- 删除便签时按块内引用回收孤儿附件（启动时兜底扫描比对）。

## 历史数据迁移（v1 notes.json → 新结构）

迁移在启动时、任何读取之前自动执行；**幂等、可中断恢复、不丢数据**：

1. **触发条件**：`notes.json` 存在且 `notes/` 目录不存在（或为空）→ 执行；否则跳过。新装用户无 `notes.json`，直接走新结构。
2. **逐 Note 写入**：解析 `notes.json`（复用现有解析；损坏则沿用现有 corrupt 备份逻辑，空启动）。逐个 Note 原子写（tmp+rename）`notes/<id>.json`。单文件失败不中断整体：记录日志并继续，保证已成功的便签落盘。
3. **完成标记**：全部 Note 写入成功后，将 `notes.json` 重命名为 `notes.json.migrated-<yyyyMMdd-HHmmss>`（**保留而非删除**，供回滚与诊断；迁移后首次确认稳定再考虑清理）。
4. **中断恢复**：进程在步骤 2 中途崩溃 → 下次启动时 `notes.json` 仍在，重新执行迁移；已写的 `<id>.json` 原子覆盖（幂等）。启动后 `notes/` 已有文件而 `notes.json` 亦在 → 仍以 `notes.json` 为准重新迁移（单进程应用，不存在 notes.json 被并发修改的情况）。
5. **迁移范围**：当前 v1 数据均为文本块，无媒体。data URL 内嵌媒体不在本期迁移范围（`uploadFile` 尚未启用，实际不存在该类数据）；若未来出现，另行版本迁移（从块中提取 base64 → 附件文件 → 替换引用）。
6. **数据目录切换（SetDataDir）**：随新结构改为整目录迁移（拷贝 `notes/`、`attachments/`、`settings.json`），相对引用保证切换后无需改写块内路径。

## 前端接口兼容

`bridge.ts` 的 `loadNotes()/saveNotes(notes[])` 全量接口**保持不变**：Go 侧对全量列表与内存索引做 diff，识别新增/修改/删除并增量落盘。前端零改动。

**Considered Options**
- SQLite 主存储（`modernc.org/sqlite` 纯 Go 驱动）：事务/FTS5/单文件备份强，但 OneDrive 同步目录下整体损坏风险（SQLite 官方警告 + WAL 伴随文件），二进制数据不可移植，方向偏离 ADR-0002 既定形态。数据量远未到需要数据库的程度。不选。
- 保持单文件 JSON + 优化：读写放大与单点风险不解决，媒体 base64 后更糟。不选。
- base64 内嵌媒体：单文件自包含，但体积膨胀 33%、全量重写放大、OneDrive 同步大文件。不选。
- 混合（文件为主 + SQLite 派生索引，Obsidian 模式）：健壮性最优，但引入 SQLite 依赖为时过早。推迟到数据量/搜索需求出现。

**Consequences**
- store.go 重构：`notesPath()` 单文件路径 → `notes/` 目录操作 + 启动扫描 + diff 增量写；新增 `SaveAttachment` binding；新增启动迁移逻辑与对应测试（含中断恢复、幂等、损坏文件场景）。
- main.go 新增 AssetServer 中间件；dev 模式（Vite :9245）需为 `/attachments/*` 加代理或退化为 base64 显示。
- settings.json 存储与读写保持不变。
- 内存索引与磁盘的一致性由单进程 + 每次变更即时落盘保证；不引入文件监听（便签场景无外部编辑需求）。
