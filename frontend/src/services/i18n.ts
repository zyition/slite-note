/**
 * Centralized UI strings, English-first (per decision). Adding a locale
 * means adding a translations object with the same keys — the keys stay
 * stable. UI reads through the dynamic `t` proxy, so components can keep
 * `import { t } from "./i18n"` and still re-render in the new language
 * after setLocale (the app subscribes via useLocale).
 */

import { useSyncExternalStore } from "react";

const en = {
  appName: "Slite Note",
  untitled: "Untitled",
  newNote: "New note",
  notes: "Notes",
  alwaysOnTop: "Always on top",
  theme: "Theme",
  themeFollowsOs: "Follows the OS",
  hide: "Hide",
  close: "Hide to tray",
  settings: "Settings",
  placeholder: "Type '/' for commands…",
  saveFailed: "Failed to save note",
  loadFailed: "Failed to load notes",

  // Editor right-click menu (see components/EditorContextMenu.tsx)
  menuCut: "Cut",
  menuCopy: "Copy",
  menuPaste: "Paste",
  menuPastePlain: "Paste as plain text",
  menuSelectAll: "Select all",
  menuCopyImage: "Copy image",
  menuDeleteBlock: "Delete block",

  // Note picker: delete / rename
  deleteNote: "Delete note",
  deleteConfirm: "Delete?",
  renameNote: "Rename note",
  exportNote: "Export as Markdown",
  importMarkdown: "Import Markdown…",
  // Settings panel
  settingsTitle: "Settings",
  closePanel: "Close settings",
  hotkeySection: "Global shortcut",
  hotkeyDesc: "Press the key combo that shows/hides the window from anywhere.",
  changeHotkey: "Change",
  pressNewHotkey: "Press the new shortcut…",
  hotkeyChangeFailed: "Could not set that shortcut",
  autoStartSection: "Launch at startup",
  autoStartDesc: "Start Slite Note automatically when you sign in to your computer.",
  dataSection: "Data location",
  dataDesc:
    "Notes and settings are stored in this folder. Move them to a new location, or point Slite Note at an existing one.",
  openExplorer: "Open in Explorer",
  changeLocationTitle: "Move data to another folder",
  moveData: "Change location…",
  migrateDone: "Done — notes moved, Slite Note now uses the new folder.",
  useExisting: "Use existing…",
  useExistingDone: "Done — Slite Note now reads notes from the selected folder.",
  nativeOnly: "Only available in the desktop app.",
  cancel: "Cancel",
  opacitySection: "Window opacity",
  // macOS fades the note background (text stays crisp); Windows fades the
  // whole window. The single line covers both readings.
  opacityDesc: "Make the note see-through. 100% is fully opaque.",
  on: "On",
  off: "Off",

  // Interface font size (settings)
  uiScaleSection: "Interface font size",
  uiScaleDesc: "Adjust the size of app interface text. Note content stays as-is.",
  uiScaleSmall: "Small",
  uiScaleMedium: "Medium",
  uiScaleLarge: "Large",

  // Markdown export (settings)
  exportAllSection: "Markdown export",
  exportAllDesc: "Write every note as its own .md file into a folder of your choice.",
  exportAll: "Export all as Markdown…",
  exportDone: (n: number) => `Exported ${n} note${n === 1 ? "" : "s"} to the selected folder.`,

  // Attachments (settings)
  attachmentsSection: "Attachments",
  attachmentsDesc:
    "Pasted images are stored as files in the data folder. Remove blobs that no note references.",
  autoCleanAttachments: "Clean up on launch",
  autoCleanDesc: "Remove unreferenced attachment files when Slite Note starts.",
  cleanNow: "Clean now",
  cleanConfirm:
    "This permanently deletes attachment files that no note references. Undo cannot restore them. Continue?",
  cleanOrphans: (n: number) => `Removed ${n} unreferenced attachment${n === 1 ? "" : "s"}.`,
  cleanNoOrphans: "No unreferenced attachments found.",

  // Shortcut cheatsheet
  shortcutsTitle: "Keyboard shortcuts",
  shortcutGroupGlobal: "Global",
  shortcutGroupApp: "App",
  shortcutGroupFormatting: "Formatting",
  sToggleWindow: "Show / Hide Slite Note",
  sNextNote: "Next note",
  sPrevNote: "Previous note",
  sNewNote: "New note",
  sOpenSettings: "Open settings",
  sCycleTheme: "Cycle theme",
  sShowShortcuts: "Show shortcut cheatsheet",
  sBold: "Bold",
  sItalic: "Italic",
  sStrike: "Strikethrough",
  sInlineCode: "Inline code",
  sCheckItem: "Toggle checkbox",
  sUndo: "Undo",
  sRedo: "Redo",

  // About section
  aboutSection: "About",
  versionLabel: "Version",
  homepageLabel: "Homepage",
  licenseLabel: "License",
  aboutDesc:
    "Slite Note. A minimal sticky note for your desktop. Built with Wails, BlockNote, React, Tailwind and lucide.",

  // Theme names (also used by services/theme.ts)
  themeDark: "Dark",
  themeGray: "Gray",
  themeYellow: "Yellow",
  /** Full label for the "System" theme row, e.g. "System (Dark)". */
  themeSystemLabel: (name: string) => `System (${name})`,

  // Language selection (title-bar picker)
  language: "Language",
  langSystem: "System",
  langEnglish: "English",
  langChinese: "简体中文",
  /** System row shows the resolved language, e.g. "System (English)". */
  langSystemLabel: (name: string) => `System (${name})`,
};

const zhCN: typeof en = {
  appName: "Slite Note",
  untitled: "无标题",
  newNote: "新建便签",
  notes: "便签",
  alwaysOnTop: "置顶",
  theme: "主题",
  themeFollowsOs: "跟随系统",
  hide: "隐藏",
  close: "隐藏到托盘",
  settings: "设置",
  placeholder: "输入 '/' 查看命令…",
  saveFailed: "保存便签失败",
  loadFailed: "加载便签失败",

  menuCut: "剪切",
  menuCopy: "复制",
  menuPaste: "粘贴",
  menuPastePlain: "粘贴为纯文本",
  menuSelectAll: "全选",
  menuCopyImage: "复制图片",
  menuDeleteBlock: "删除块",

  deleteNote: "删除便签",
  deleteConfirm: "删除？",
  renameNote: "重命名",
  exportNote: "导出 Markdown",
  importMarkdown: "导入 Markdown…",

  settingsTitle: "设置",
  closePanel: "关闭设置",
  hotkeySection: "全局快捷键",
  hotkeyDesc: "按组合键可在任意位置显示/隐藏窗口。",
  changeHotkey: "更改",
  pressNewHotkey: "请按下新的快捷键…",
  hotkeyChangeFailed: "无法设置该快捷键",
  autoStartSection: "开机启动",
  autoStartDesc: "登录电脑后自动启动 Slite Note。",
  dataSection: "数据位置",
  dataDesc: "便签与设置保存在此文件夹中。可移动到新位置，或让 Slite Note 使用已有文件夹。",
  openExplorer: "在资源管理器中打开",
  changeLocationTitle: "移动数据到其他文件夹",
  moveData: "更改位置…",
  migrateDone: "完成——便签已移动，Slite Note 现使用新文件夹。",
  useExisting: "使用已有…",
  useExistingDone: "完成——Slite Note 现读取所选文件夹中的便签。",
  nativeOnly: "仅在桌面应用中可用。",
  cancel: "取消",
  opacitySection: "窗口不透明度",
  opacityDesc: "让便签呈半透明，100% 为完全不透明。",
  on: "开",
  off: "关",

  uiScaleSection: "界面字体大小",
  uiScaleDesc: "调整应用界面文字的大小。便签正文不受影响。",
  uiScaleSmall: "小",
  uiScaleMedium: "中",
  uiScaleLarge: "大",

  exportAllSection: "导出 Markdown",
  exportAllDesc: "将每条便签导出为独立 .md 文件到所选文件夹。",
  exportAll: "全部导出为 Markdown…",
  exportDone: (n: number) => `已将 ${n} 条便签导出到所选文件夹。`,

  // Attachments (settings)
  attachmentsSection: "附件",
  attachmentsDesc: "粘贴的图片作为文件保存在数据文件夹中。删除不再被任何便签引用的附件。",
  autoCleanAttachments: "启动时清理",
  autoCleanDesc: "Slite Note 启动时删除未被引用的附件文件。",
  cleanNow: "立即清理",
  cleanConfirm: "这将永久删除不再被任何便签引用的附件文件，撤销无法恢复。继续？",
  cleanOrphans: (n: number) => `已删除 ${n} 个未引用附件。`,
  cleanNoOrphans: "未找到未引用的附件。",

  shortcutsTitle: "键盘快捷键",
  shortcutGroupGlobal: "全局",
  shortcutGroupApp: "应用",
  shortcutGroupFormatting: "格式",
  sToggleWindow: "显示 / 隐藏 Slite Note",
  sNextNote: "下一条便签",
  sPrevNote: "上一条便签",
  sNewNote: "新建便签",
  sOpenSettings: "打开设置",
  sCycleTheme: "切换主题",
  sShowShortcuts: "显示快捷键速查表",
  sBold: "加粗",
  sItalic: "斜体",
  sStrike: "删除线",
  sInlineCode: "行内代码",
  sCheckItem: "切换复选框",
  sUndo: "撤销",
  sRedo: "重做",

  aboutSection: "关于",
  versionLabel: "版本",
  homepageLabel: "主页",
  licenseLabel: "许可证",
  aboutDesc:
    "Slite Note。一款极简桌面便签应用，基于 Wails、BlockNote、React、Tailwind 与 lucide 构建。",

  themeDark: "深色",
  themeGray: "灰色",
  themeYellow: "黄色",
  themeSystemLabel: (name: string) => `系统（${name}）`,

  language: "语言",
  langSystem: "跟随系统",
  langEnglish: "English",
  langChinese: "简体中文",
  langSystemLabel: (name: string) => `跟随系统（${name}）`,
};

export type Locale = "en" | "zh-CN";
export type Messages = typeof en;

const locales: Record<Locale, Messages> = { en, "zh-CN": zhCN };

/** Map a system language (navigator.language) to a supported locale. */
export function resolveLocale(systemLang: string): Locale {
  return systemLang.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

/**
 * Resolve a persisted language choice to the active locale. "system" (or
 * any unknown/legacy value, e.g. "" from older settings) follows the
 * OS/browser language; a concrete choice is used as-is.
 */
export function resolveChoice(choice: string): Locale {
  if (choice === "en" || choice === "zh-CN") return choice;
  const sys = typeof navigator !== "undefined" ? navigator.language : "en";
  return resolveLocale(sys);
}

let current: Locale = "en";
const listeners = new Set<() => void>();

/** Switch the active UI language; subscribed components re-render. */
export function setLocale(locale: Locale): void {
  if (locale === current) return;
  current = locale;
  for (const l of listeners) l();
}

export function getLocale(): Locale {
  return current;
}

/** React hook — subscribe a component to language changes. */
export function useLocale(): Locale {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb);
      return () => {
        listeners.delete(cb);
      };
    },
    getLocale,
    getLocale,
  );
}

/** Dynamic message lookup — components keep `import { t } from "./i18n"`. */
export const t: Messages = new Proxy({} as Messages, {
  get: (_target, prop: keyof Messages) => locales[current][prop],
}) as Messages;
