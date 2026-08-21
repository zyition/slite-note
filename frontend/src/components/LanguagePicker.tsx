import { useEffect, useRef, useState } from "react";
import { Check, Languages } from "lucide-react";
import type { LanguageName } from "../types/note";
import { LANGUAGES } from "../types/note";
import { t, useLocale } from "../services/i18n";

interface LanguagePickerProps {
  /** User's persisted choice ("system" included). */
  choice: LanguageName;
  /** Apply + persist a new choice. */
  onSelect: (choice: LanguageName) => void;
  /** Notifies the app while the popover is up (window stays opaque). */
  onOpenChange?: (open: boolean) => void;
}

/**
 * LanguagePicker — the title-bar language control. The trigger shows the
 * Languages icon; its tooltip names the active language. Clicking opens a
 * compact vertical list: System (with the currently resolved language),
 * English, 简体中文. Language names are shown in their own language.
 *
 * Styling mirrors the ThemePicker / settings dialog (theme vars, radius,
 * shadow, 11px UI type).
 */
export function LanguagePicker({ choice, onSelect, onOpenChange }: LanguagePickerProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  // Re-render when the active language changes (e.g. after switching) so the
  // System row and tooltip show the resolved language immediately.
  const locale = useLocale();

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

  // The active language's own name (English / 简体中文) — identical in both
  // locales, so t.langEnglish / t.langChinese work regardless of UI language.
  const currentName = locale === "zh-CN" ? t.langChinese : t.langEnglish;
  const systemLabel = t.langSystemLabel(currentName);
  const tooltip = `${t.language}: ${
    choice === "system" ? systemLabel : choice === "zh-CN" ? t.langChinese : t.langEnglish
  }`;

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
        <Languages size={13} />
      </button>

      {open && (
        <div
          role="menu"
          aria-label={t.language}
          className="absolute right-0 top-full z-[90] mt-1 w-44 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--bg)] py-1 shadow-lg"
        >
          {LANGUAGES.map((value) => {
            const selected = choice === value;
            const label =
              value === "system"
                ? systemLabel
                : value === "en"
                  ? t.langEnglish
                  : t.langChinese;
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
                <span className="min-w-0 flex-1 leading-tight">{label}</span>
                {selected && <Check size={12} className="shrink-0 text-[var(--accent)]" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
