# 单实例保证 + NSIS 安装器升级加固 + 产品命名

0.1.0 发布后暴露三个问题：(1) 应用无单实例保证，双开会对 `notes.json` 并发写（Store 的 mutex 只在单进程内有效）；(2) Wails 默认 NSIS 安装器覆盖安装时，旧版本正在运行会因 exe 文件锁直接报错中止，且不记录 InstallLocation、不跑旧卸载器、不比较版本；(3) 安装产物（快捷方式、安装目录、卸载显示名）用 `slite-note` 小写连字符，不符合产品命名惯例。此外发现 release CI 只改写 `info.json` 版本，`wails_tools.nsh` 的 `INFO_PRODUCTVERSION` 写死 0.1.0，导致安装器与 ARP 注册表 `DisplayVersion` 永远不随 release 更新。

## 决策

1. **单实例**：使用 Wails v3 内置 `application.Options.SingleInstance`（Windows 实现 = 命名 Mutex + 隐藏 message-only 窗口 + WM_COPYDATA 透传 argv）。第二实例退出码 0；`OnSecondInstanceLaunch` 依据 argv 分流：`--quit` → 通知第一实例 flush 自动保存后 `app.Quit()`（安装器升级前置调用，替代强杀防丢数据）；`--silent` → 忽略（防 autostart 与手动启动竞态弹窗）；无参数 → `showMainWindow()` 唤出窗口（契合 hide/summon 模型）。
2. **NSIS 加固**（`build/windows/nsis/project.nsi` 为模板 fork，`wails3 build` 不覆盖，`update:build-assets` 会覆盖须规避）：
   - 卸载键名固定为无空格 `zyitionSliteNote`（与显示名解耦，改名不丢升级链），兼容读取旧键 `zyitionslite-note` 做迁移升级；
   - `.onInit` 定位上一版本（新键 → HKLM → 旧键），升级原位安装（`$INSTDIR` 复用旧目录），`InstallLocation` 落注册表；
   - 升级先复制旧卸载器到 `$PLUGINSDIR` 再 `ExecWait /S _?=$INSTDIR`（`_?=` 不引号、末位，同步等待且防自删）；
   - 安装/卸载前 `tasklist` 检测 `slite-note.exe`（零插件依赖），交互弹窗征询、静默走 `--quit` 优雅退出（≤10s 超时 `taskkill /F` 兜底）；
   - 交互模式 `VersionCompare` 阻止降级；静默模式 `/SD IDYES` 直接覆盖（winget 语义）；
   - 卸载仅删 WebView2 缓存与安装目录，**保留用户数据**（`%APPDATA%\slite`）。
3. **命名**：`build/config.yml` / `info.json` `productName` 改 `Slite Note`（exe 文件名保持 `slite-note.exe` 不变），`project.nsi` 中 `INFO_PRODUCTNAME` 覆盖生效 → 快捷方式 `Slite Note.lnk`、安装目录 `Programs\Slite Note`、ARP `DisplayName` 全部对齐。
4. **版本传递**：`windows:package` 支持 `VERSION` 变量透传 `-DINFO_PRODUCTVERSION`，release.yml 传 tag 版本。

**Considered Options**
- 自写 CreateMutex 单例：Wails 已内置且带 argv 透传，重复造轮子。
- 安装器直接 `taskkill /F`：丢未保存输入，与 `--quit` 优雅退出相比风险高。
- 卸载键沿用 `${COMPANY}${PRODUCT}` 拼接：改名（slite-note → Slite Note）会改变键名使升级链断裂，故固定。
- `nsProcess` 插件检测进程：choco NSIS 未必自带，`tasklist` + StrStr 零额外依赖。

**Consequences**
- 单实例后无法通过双开对比数据，但数据一致性收益更高；第二实例无参数唤出窗口成为新的启动路径。
- 升级路径依赖 ARP 键稳定；未来若迁移到 winget/Inno/Velopack，`InstallLocation` 与稳定键名已就绪。
- `project.nsi` 与 `wails_tools.nsh` 分离维护：`wails3 update build-assets` 会覆盖前者，CI 与文档均已标注规避。
- 交互安装多一次"关闭运行中的应用"确认弹窗；静默（winget/CI）路径无感。
