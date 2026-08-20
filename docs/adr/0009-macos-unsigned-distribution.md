# macOS 未签名分发（侧载 .app / dmg）

## 背景

维护者无 Apple Developer 账号（个人使用场景，不付费订阅）。macOS 的 Gatekeeper 默认拦截未签名/未公证的应用：首次打开会提示"无法验证开发者"，需要右键 → 打开，或 `xargs xattr -cr` 移除隔离属性。

目标形态是"用户可直接从 GitHub Releases 下载安装"，无开发者账号、无公证，即 **Q1=B 未签名侧载**。

## 决策

- **产物**：Universal Binary（arm64 + amd64，`lipo` 合并）+ 样式化 `.dmg`，经 GitHub Actions `macos-latest` runner 构建（`darwin:package:universal:dmg`，新增 task）。
- **签名**：ad-hoc 签名（`codesign --force --deep --sign -`，Taskfile `codesign:adhoc`），保证 app 在本地跑通且满足系统基本校验；不做 Developer ID 签名、不公证。
- **发布渠道**：与 Windows 并列挂 GitHub Releases（`slite-note.dmg`）。
- **用户引导**：README 写明侧载步骤（右键 → 打开；或 `xattr -cr /Applications/slite-note.app`）；Gatekeeper 警告是预期行为，不是损坏。
- **版本**：tag 注入 `Info.plist`（`CFBundleShortVersionString`/`CFBundleVersion`，`plutil`）+ Go 二进制（`-X main.appVersion`）。

## Consequences

- 首次打开有 Gatekeeper 弹窗，需要文档引导 —— 无账号侧载的固有代价，接受。
- 无自动更新、无公证的"已通过 Apple 检查"背书；安全提示由用户自行判断。
- 将来若注册开发者账号：CI 补 Developer ID 签名 + `notarize` 即可无缝升级（Taskfile 已预留 `sign` / `sign:notarize` task），产物形态不变。
- 冒烟测试（`--smoke`）在 CI mac job 中验证裸二进制可启动，降低"打不开"风险。
