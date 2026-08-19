import { useEffect, useRef, useState } from "react";
import { Check, Palette } from "lucide-react";
import type { ThemeName } from "../types/note";
import { THEME_NAMES } from "../types/note";
import { themeName, themeSwatch } from "../services/theme";
import { t } from "../services/i18n";

interface ThemePickerProps {
  /** User's persisted choice ("system" included). */
  choice: ThemeName;
  /** Concrete theme actually in effect (resolveTheme output). */
  applied: Exclude<ThemeName, "system">;
  /** Apply + persist a new choice. */
  onSelect: (choice: ThemeName) => void;
  /** Notifies the app while the popover is up (window stays opaque). */
  onOpenChange?: (open: boolean) => void;
}

/**
 * ThemePicker — the title-bar theme control. The trigger shows the current
 * theme's single-letter badge (D/G/Y, or the resolved one under "system")
 * and its tooltip names the active theme. Clicking opens a compact vertical
 * list: System (with its live OS-following target), Dark, Gray, Yellow.
 *
 * Styling mirrors the Settings dialog (theme vars, radius, shadow, 11px UI
 * type). The list is intentionally a vertical list so more options can be
 * added later without a layout change.
 */
export function ThemePicker({ choice, applied, onSelect, onOpenChange }: ThemePickerProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Let the app lift window opacity while the popover is open.
  useEffect(() => {
    onOpenChange?.(open);
  }, [open, onOpenChange]);

  // Close on outside click or Escape.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const systemLabel = `System (${themeName(applied)})`;
  const tooltip = `${t.theme}: ${choice === "system" ? systemLabel : themeName(applied)}`;

  return (
    <div ref={rootRef} className="no-drag relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        title={tooltip}
        aria-label={tooltip}
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex h-6 w-6 items-center justify-center rounded text-[var(--fg-muted)] hover:bg-[var(--hover)] hover:text-[var(--fg)]"
      >
        <Palette size={13} />
      </button>

      {open && (
        <div
          role="menu"
          aria-label={t.theme}
          className="absolute right-0 top-full z-[90] mt-1 w-44 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--bg)] py-1 shadow-lg"
        >
          {THEME_NAMES.map((value) => {
            const isSystem = value === "system";
            const concrete = isSystem ? applied : value;
            const selected = choice === value;
            return (
              <button
                key={value}
                type="button"
                role="menuitemradio"
                aria-checked={selected}
                onClick={() => {
                  onSelect(value);
                  setOpen(false);
                }}
                className="flex w-full items-center gap-2 px-2.5 py-1.5 text-left text-[11px] text-[var(--fg)] hover:bg-[var(--hover)]"
              >
                <span
                  className="h-3.5 w-3.5 shrink-0 rounded border border-[var(--border)]"
                  style={{ background: themeSwatch(concrete) }}
                />
                <span className="min-w-0 flex-1 leading-tight">
                  <span className="block truncate">{isSystem ? systemLabel : themeName(concrete)}</span>
                  {isSystem && (
                    <span className="block text-[9px] text-[var(--fg-muted)]">{t.themeFollowsOs}</span>
                  )}
                </span>
                {selected && <Check size={12} className="shrink-0 text-[var(--accent)]" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
