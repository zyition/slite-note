import { useEffect, useRef, useState } from "react";
import { Check, FolderOpen, Keyboard, Loader2, Power, X } from "lucide-react";
import type { Settings } from "../types/note";
import { t } from "../services/i18n";
import { formatCombo, displayCombo } from "../services/hotkey";
import {
  currentDataDir,
  openDataDir,
  setDataDir,
  setHotkey,
  validateDataDir,
} from "../services/bridge";

interface SettingsPanelProps {
  open: boolean;
  settings: Settings;
  onClose: () => void;
  /** Persist a settings change (App updates state + saves). */
  onChanged: (next: Settings) => void;
}

type DirCheck =
  | { status: "idle" }
  | { status: "checking" }
  | { status: "ok" }
  | { status: "error"; message: string };

/**
 * SettingsPanel — modal overlay reached from the title bar gear or the tray
 * "Settings…" entry. Sections: global shortcut (record a combo), launch at
 * startup, and data location (reveal / pre-check / migrate).
 */
export function SettingsPanel({ open, settings, onClose, onChanged }: SettingsPanelProps) {
  const [recording, setRecording] = useState(false);
  const [hotkeyError, setHotkeyError] = useState<string | null>(null);
  const [dataDir, setDataDirDisplay] = useState(settings.dataDir || "…");
  const [dirInput, setDirInput] = useState("");
  const [dirCheck, setDirCheck] = useState<DirCheck>({ status: "idle" });
  const [migrating, setMigrating] = useState(false);
  const [migrated, setMigrated] = useState(false);

  // Refresh the displayed directory whenever the panel opens (the value may
  // have changed after a migration).
  useEffect(() => {
    if (!open) return;
    setHotkeyError(null);
    setMigrated(false);
    setDirCheck({ status: "idle" });
    setDirInput("");
    currentDataDir()
      .then(setDataDirDisplay)
      .catch(() => {});
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !recording) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, recording, onClose]);

  // While recording, capture every keydown at the window level so the combo
  // works regardless of which element has focus.
  const recordHandler = useRef<(e: KeyboardEvent) => void>(() => {});
  useEffect(() => {
    recordHandler.current = async (e: KeyboardEvent) => {
      if (!recording) return;
      e.preventDefault();
      e.stopPropagation();
      if (e.key === "Escape") {
        setRecording(false);
        return;
      }
      const combo = formatCombo(e);
      if (!combo) return; // bare modifier / unsupported key: keep waiting
      setRecording(false);
      try {
        await setHotkey(combo);
        onChanged({ ...settings, hotkey: combo });
      } catch (err) {
        setHotkeyError(String((err as Error)?.message ?? err));
      }
    };
  });
  useEffect(() => {
    if (!recording) return;
    const h = (e: KeyboardEvent) => void recordHandler.current(e);
    window.addEventListener("keydown", h, true);
    return () => window.removeEventListener("keydown", h, true);
  }, [recording]);

  /* ---------- hotkey recording ---------- */

  const beginRecording = () => {
    setRecording(true);
    setHotkeyError(null);
  };


  if (!open) return null;

  /* ---------- data dir ---------- */

  const handleCheck = async () => {
    setDirCheck({ status: "checking" });
    const err = await validateDataDir(dirInput);
    setDirCheck(err ? { status: "error", message: err } : { status: "ok" });
  };

  const handleMigrate = async () => {
    setMigrating(true);
    const err = await setDataDir(dirInput);
    setMigrating(false);
    if (err) {
      setDirCheck({ status: "error", message: err });
      return;
    }
    const dir = await currentDataDir().catch(() => "");
    if (dir) setDataDirDisplay(dir);
    setMigrated(true);
    setDirCheck({ status: "idle" });
  };

  const sectionLabel =
    "mb-1.5 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-[var(--fg-muted)]";
  const inputCls =
    "w-full rounded border border-[var(--border)] bg-[var(--bg-input)] px-2 py-1.5 text-[11px] outline-none focus:border-[var(--accent)]";
  const primaryBtn =
    "rounded bg-[var(--accent)] px-2.5 py-1.5 text-[11px] font-medium text-[var(--accent-fg)] hover:opacity-90 disabled:opacity-50";

  return (
    <div
      className="fixed inset-0 z-[100] flex items-start justify-center bg-black/30 p-4 pt-14"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      {/* While recording, show a dim hint overlay so the user knows a combo
          is being captured. */}
      {recording && (
        <div className="pointer-events-none fixed inset-0 z-[102] flex items-start justify-center pt-24">
          <span className="rounded-full bg-[var(--accent)] px-3 py-1.5 text-[11px] font-medium text-[var(--accent-fg)] shadow">
            {t.pressNewHotkey}
          </span>
        </div>
      )}

      <div
        className="w-full max-w-80 rounded-lg border border-[var(--border)] bg-[var(--bg)] shadow-xl"
        role="dialog"
        aria-modal="true"
        aria-label={t.settingsTitle}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--border)] px-3 py-2">
          <h2 className="text-[12px] font-semibold">{t.settingsTitle}</h2>
          <button
            className="rounded p-1 text-[var(--fg-muted)] hover:bg-[var(--hover)]"
            onClick={onClose}
            title={t.closePanel}
          >
            <X size={13} />
          </button>
        </div>

        <div className="max-h-[70vh] space-y-4 overflow-y-auto px-3 py-3">
          {/* Global shortcut */}
          <section>
            <div className={sectionLabel}>
              <Keyboard size={11} /> {t.hotkeySection}
            </div>
            <p className="mb-2 text-[10px] leading-snug text-[var(--fg-muted)]">{t.hotkeyDesc}</p>
            <div className="flex items-center gap-2">
              {recording ? (
                <span className="flex-1 rounded border border-dashed border-[var(--accent)] px-2 py-1.5 text-[11px] text-[var(--accent)]">
                  {t.pressNewHotkey}
                </span>
              ) : (
                <kbd className="flex-1 rounded border border-[var(--border)] bg-[var(--bg-input)] px-2 py-1.5 text-[11px] font-semibold">
                  {displayCombo(settings.hotkey)}
                </kbd>
              )}
              <button
                className={primaryBtn}
                onClick={recording ? () => setRecording(false) : beginRecording}
                disabled={recording}
              >
                {recording ? t.cancel : t.changeHotkey}
              </button>
            </div>
            {hotkeyError && <p className="mt-1.5 text-[10px] text-red-500">{hotkeyError}</p>}
          </section>

          {/* Launch at startup */}
          <section>
            <div className={sectionLabel}>
              <Power size={11} /> {t.autoStartSection}
            </div>
            <p className="mb-2 text-[10px] leading-snug text-[var(--fg-muted)]">{t.autoStartDesc}</p>
            <button
              role="switch"
              aria-checked={settings.launchAtStartup}
              onClick={() => onChanged({ ...settings, launchAtStartup: !settings.launchAtStartup })}
              className={`relative h-5 w-9 rounded-full transition-colors ${
                settings.launchAtStartup ? "bg-[var(--accent)]" : "bg-[var(--border)]"
              }`}
            >
              <span
                className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-all ${
                  settings.launchAtStartup ? "left-[18px]" : "left-0.5"
                }`}
              />
            </button>
            <span className="ml-2 align-middle text-[11px]">
              {settings.launchAtStartup ? "On" : "Off"}
            </span>
          </section>

          {/* Data location */}
          <section>
            <div className={sectionLabel}>
              <FolderOpen size={11} /> {t.dataSection}
            </div>
            <p className="mb-2 text-[10px] leading-snug text-[var(--fg-muted)]">{t.dataDesc}</p>
            <div className="mb-1.5 break-all rounded border border-[var(--border)] bg-[var(--bg-input)] px-2 py-1.5 font-mono text-[10px]">
              {dataDir}
            </div>
            <button className="text-[11px] text-[var(--accent)] hover:underline" onClick={() => void openDataDir()}>
              {t.openExplorer}
            </button>

            <div className="mt-3 border-t border-[var(--border)] pt-3">
              <div className="mb-1.5 text-[11px] font-medium">{t.changeLocationTitle}</div>
              <div className="flex gap-1.5">
                <input
                  className={inputCls}
                  value={dirInput}
                  onChange={(e) => setDirInput(e.target.value)}
                  placeholder={t.targetDirPlaceholder}
                  spellCheck={false}
                />
                <button className={primaryBtn} onClick={() => void handleCheck()} disabled={!dirInput.trim()}>
                  {dirCheck.status === "checking" ? <Loader2 size={11} className="animate-spin" /> : t.checkDir}
                </button>
              </div>

              {dirCheck.status === "ok" && (
                <div className="mt-2 flex items-center justify-between gap-2">
                  <span className="flex items-center gap-1 text-[10px] text-green-600 dark:text-green-400">
                    <Check size={11} /> {t.checkDirOk}
                  </span>
                  <button className={primaryBtn} onClick={() => void handleMigrate()} disabled={migrating}>
                    {migrating ? <Loader2 size={11} className="animate-spin" /> : t.migrateHere}
                  </button>
                </div>
              )}
              {dirCheck.status === "error" && (
                <p className="mt-2 text-[10px] text-red-500">{dirCheck.message}</p>
              )}
              {migrated && (
                <p className="mt-2 flex items-center gap-1 text-[10px] text-green-600 dark:text-green-400">
                  <Check size={11} /> {t.migrateDone}
                </p>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
