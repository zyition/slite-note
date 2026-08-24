import { useEffect } from "react";
import { Keyboard, X } from "lucide-react";
import { t } from "../services/i18n";
import { displayParts } from "../services/hotkey";
import {
  appShortcuts,
  SHORTCUT_GROUPS,
  type ShortcutDef,
  type ShortcutGroup,
} from "../services/shortcuts";

interface ShortcutsPanelProps {
  open: boolean;
  /** Current global toggle hotkey (shown in the Global group). */
  hotkey: string;
  onClose: () => void;
}

/**
 * ShortcutsPanel — modal cheatsheet reached from the title-bar keyboard icon
 * or Ctrl+Shift+/. Lists the global toggle hotkey and the app-level / inline
 * formatting shortcuts. BlockNote's block shortcuts (Ctrl+Alt+1 …) live in
 * the slash menu as badges, so they are not repeated here.
 *
 * While open, the app keeps the window fully opaque (App lifts opacity when
 * `shortcutsOpen` is set), matching the settings panel.
 */
export function ShortcutsPanel({ open, hotkey, onClose }: ShortcutsPanelProps) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  /** Global row is dynamic (the user-configurable toggle hotkey). */
  const rowsFor = (group: ShortcutGroup): ShortcutDef[] => {
    const rows = appShortcuts().filter((s) => s.group === group);
    if (group === "global") {
      rows.unshift({
        id: "toggle-window",
        keys: [hotkey || "Alt+Shift+S"],
        label: t.sToggleWindow,
        group: "global",
      });
    }
    return rows;
  };

  /** Group heading text, resolved from the live i18n proxy per render. */
  const groupLabel = (group: ShortcutGroup): string => {
    switch (group) {
      case "global":
        return t.shortcutGroupGlobal;
      case "app":
        return t.shortcutGroupApp;
      case "formatting":
        return t.shortcutGroupFormatting;
    }
  };

  const iconBtn =
    "inline-flex h-[length:var(--btn-h)] w-[length:var(--btn-h)] shrink-0 items-center justify-center rounded-md text-[var(--fg-muted)] transition-colors duration-150 hover:bg-[var(--hover)] hover:text-[var(--fg)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-1 focus-visible:ring-offset-[var(--bg)] [&_svg]:h-[length:var(--icon-sm)] [&_svg]:w-[length:var(--icon-sm)]";

  return (
    <div
      className="fixed inset-0 z-[100] flex items-start justify-center bg-black/45 p-4 pt-14"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="w-full max-w-[340px] rounded-lg border border-[var(--border)] bg-[var(--bg)] shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-label={t.shortcutsTitle}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--border)] px-3 py-2">
          <h2 className="flex items-center gap-1.5 text-[length:var(--fs-title)] font-semibold">
            <Keyboard size={13} className="h-[length:var(--icon-md)] w-[length:var(--icon-md)]" /> {t.shortcutsTitle}
          </h2>
          <button
            className={iconBtn}
            onClick={onClose}
            title={t.closePanel}
            aria-label={t.closePanel}
          >
            <X size={13} className="h-[length:var(--icon-md)] w-[length:var(--icon-md)]" />
          </button>
        </div>

        <div className="max-h-[78vh] space-y-3 overflow-y-auto px-3 py-3">
          {SHORTCUT_GROUPS.map((group) => {
            const rows = rowsFor(group);
            if (rows.length === 0) return null;
            return (
              <section key={group}>
                <h3 className="mb-1 text-[length:var(--fs-body)] font-semibold uppercase tracking-wide text-[var(--fg-muted)]">
                  {groupLabel(group)}
                </h3>
                <ul className="space-y-1">
                  {rows.map((row) => (
                    <li
                      key={row.id}
                      className="flex items-center justify-between gap-3 py-0.5 text-[length:var(--fs-body)] text-[var(--fg)]"
                    >
                      <span className="min-w-0 leading-tight">{row.label}</span>
                      <span className="flex shrink-0 items-center gap-1.5">
                        {row.keys.map((combo, i) => (
                          <span key={`${combo}-${i}`} className="flex items-center gap-1">
                            {i > 0 && <span className="text-[var(--fg-muted)]">·</span>}
                            {displayParts(combo).map((part) => (
                              <kbd key={part} className="slite-kbd">
                                {part}
                              </kbd>
                            ))}
                          </span>
                        ))}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            );
          })}
        </div>
      </div>
    </div>
  );
}
