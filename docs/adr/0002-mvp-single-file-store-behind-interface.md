# MVP 单文件存储，面向 Vault 的 Store 抽象

用户长期目标为类 Obsidian vault 的存储形态（可配置数据目录、每便签一文件），MVP 阶段简化为单文件。为此定义 `Store` 接口（Go 侧 NoteService + 前端桥接层），MVP 实现将全部 Note 写入 `%AppData%/slite/notes.json`（`os.UserConfigDir`，规避 req.md 建议的 `./data/notes.json` 相对路径在安装到只读位置时写失败的隐患），未来 Vault 实现可无痛替换而不改前端。文件结构 `{version: 1, notes: [...]}`。

**Considered Options**
- req.md 的 `./data/notes.json` 相对应用目录：开发期可行，安装到 Program Files 等只读位置会写失败。
- Neutralino.storage：随 Neutralino 一起被 ADR-0001 弃用。
- MVP 直接做 vault：用户明确选择更小 MVP，推迟。

**Consequences**
- 单文件在便签数量大时读写放大，但 MVP 场景（个位数便签）无感。
- 数据位置当前固定不可配置，可配置化随 Vault 迭代。
