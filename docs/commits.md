# Commit Message 规范

CHANGELOG.md 与 GitHub Release 描述由 [git-cliff](https://git-cliff.org) 从
git 历史自动生成（配置见 `cliff.toml`），因此 **commit message 就是
changelog 的内容来源**。本仓库采用
[Conventional Commits](https://www.conventionalcommits.org) 规范。

## 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

- `type`：必填，小写，见下表。
- `scope`：可选，括号包裹，标明改动模块（见下文）。
- `subject`：必填，祈使句、小写开头、不超过 72 字符，**不要以句号结尾**。
- `body`：可选，空一行后写"为什么改 + 怎么改"，细节**会进入 changelog**。
- `footer`：可选，`BREAKING CHANGE: ...` 或 `Closes #12` 等。

## type 收录对照

`cliff.toml` 中的 `commit_parsers` 决定哪些 commit 进入 changelog：

| type | 进 changelog？ | 说明 |
| --- | --- | --- |
| `feat` | ✅ Features | 新功能 |
| `fix` | ✅ Bug Fixes | 缺陷修复 |
| `perf` | ✅ Performance | 性能优化 |
| `refactor` | ✅ Refactoring | 重构（行为不变） |
| `docs` | ✅ Documentation | 文档（`docs: changelog` 等内务除外） |
| `test` | ✅ Testing | 测试相关 |
| `chore` | ❌ 跳过 | 内务，不产生用户可见变化 |
| `ci` | ❌ 跳过 | CI 配置 |
| `build` | ❌ 跳过 | 构建系统 |
| `style` | ❌ 跳过 | 格式调整 |
| `revert` | ❌ 跳过 | 回滚 |

> 想让自己的一次改动被记录进 changelog，就选对 `type`；内务改动用
> `chore`/`ci`/`build`/`style` 会被自动过滤。

## scope 约定

scope 标明改动模块，小写连字符，常用值：

- `store`：数据层（store.go、设置持久化）
- `editor`：BlockNote 编辑器相关
- `tray`：托盘 / 窗口 / 热键
- `installer`：NSIS 安装器
- `ci`：CI 配置（配合 `ci:` type 时可不加 scope）
- `docs` / `chore`：一般不写 scope

## 好例子

```
feat(store): add per-note attachments directory

Store note media under notes/*.json with hash-named immutable
attachments; wire BlockNote media via relative refs + AssetServer.

Closes #12
```

```
fix(installer): gracefully quit running app before overwrite install

The previous version held a file lock that aborted silent upgrades.
Send --quit first, fall back to taskkill after a 10s timeout.
```

## 坏例子与原因

| 坏例子 | 问题 |
| --- | --- |
| `Fix duplicate tray icon` | 非小写、无 type，**不会进 changelog** |
| `wip` | 无 type/scope，会被过滤 |
| `feat: adds feature` | subject 应为祈使句 `add` |
| `chore: fix memory leak` | 用错 type，被跳过；应为 `fix(store)` |

## breaking change

破坏性变更用 `!` 标记，例如 `feat(store)!: ...` 或 body 中写
`BREAKING CHANGE: ...`。git-cliff 会在 changelog 中标注，且不会被过滤。

## AI 生成 commit 的规则

AI 辅助提交时必须遵守：

1. **格式**：严格 `type(scope): subject`，type 从表中选取。
2. **subject**：英文祈使句，小写开头，≤72 字符。
3. **body**：有实质改动逻辑时写 body（为什么改、关键行为），
   这些内容会原样进入 changelog，避免 changelog 只剩一句话。
4. **一条 commit 一个主题**：不要混多个 type（`fix: ...; chore: ...`）。
5. **不写 changelog 内务 commit**：生成 changelog 的工作流已自动 commit
   `docs: changelog for vX.Y.Z`，无需手动提交此类 commit。

## 本地预览 changelog

```bash
mise use -g git-cliff          # 安装（mise 管理）
git-cliff -o /tmp/preview.md   # 全量预览，检查 commit 是否被正确收录
```

发版流程见 `.github/workflows/release.yml`：打 tag → push → 自动生成
CHANGELOG.md、发布说明并构建产物。
