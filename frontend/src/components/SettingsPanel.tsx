import { useCallback, useEffect, useRef, useState } from "react";
import { Check, Droplets, FolderOpen, FolderSearch, Info, Keyboard, Loader2, Power, X } from "lucide-react";
import type { Settings } from "../types/note";
import { t } from "../services/i18n";
import { formatCombo, displayCombo } from "../services/hotkey";
import {
  appVersion,
  chooseDataDir,
  currentDataDir,
  openDataDir,
  openUrl,
  resumeHotkey,
  setDataDir,
  setHotkey,
  suspendHotkey,
} from "../services/bridge";

const HOME_URL = "https://github.com/zyition/slite-note";

interface SettingsPanelProps {
  open: boolean;
  settings: Settings;
  onClose: () => void;
  /** Persist a settings change (App updates state + saves). */
  onChanged: (next: Settings) => void;
}

/**
 * SettingsPanel — modal overlay reached from the title bar gear or the tray
 * "Settings…" entry. Sections: global shortcut (record a combo), launch at
 * startup, and data location (reveal / pick a folder / pre-check / migrate).
 *
 * Hotkey recording temporarily suspends the OS-level binding so pressing the
 * old combo cannot toggle the window mid-recording; a combo equal to the
 * current one is simply kept.
 */
export function SettingsPanel({ open, settings, onClose, onChanged }: SettingsPanelProps) {
  const [recording, setRecording] = useState(false);
  const [hotkeyError, setHotkeyError] = useState<string | null>(null);
  const [dataDir, setDataDirDisplay] = useState(settings.dataDir || "…");
  const [moving, setMoving] = useState(false);
  const [migrateMsg, setMigrateMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [version, setVersion] = useState("…");

  // Tracks whether the toggle hotkey is currently suspended (recording active).
  const suspendedRef = useRef(false);

  // Refresh the displayed directory whenever the panel opens (the value may
  // have changed after a migration).
  useEffect(() => {
    if (!open) return;
    setHotkeyError(null);
    setMigrateMsg(null);
    currentDataDir()
      .then(setDataDirDisplay)
      .catch(() => {});
    appVersion().then(setVersion).catch(() => {});
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !recording) handleClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, recording]);

  /* ---------- hotkey recording ---------- */

  // Ends a recording session and restores the suspended hotkey if needed.
  const stopRecording = useCallback(() => {
    setRecording(false);
    if (suspendedRef.current) {
      suspendedRef.current = false;
      resumeHotkey().catch(() => {});
    }
  }, []);

  const beginRecording = useCallback(async () => {
    setHotkeyError(null);
    try {
      await suspendHotkey();
      suspendedRef.current = true;
      setRecording(true);
    } catch (err) {
      setHotkeyError(String((err as Error)?.message ?? err));
    }
  }, []);

  const handleClose = useCallback(() => {
    stopRecording();
    onClose();
  }, [stopRecording, onClose]);

  // While recording, capture every keydown at the window level so the combo
  // works regardless of which element has focus.
  const recordHandler = useRef<(e: KeyboardEvent) => void>(() => {});
  useEffect(() => {
    recordHandler.current = async (e: KeyboardEvent) => {
      if (!recording) return;
      e.preventDefault();
      e.stopPropagation();
      if (e.key === "Escape") {
        stopRecording();
        return;
      }
      const combo = formatCombo(e);
      if (!combo) return; // bare modifier / unsupported key: keep waiting
      if (combo === settings.hotkey) {
        // Same combo: keep it as-is; resume restores the binding.
        stopRecording();
        return;
      }
      try {
        await setHotkey(combo);
        onChanged({ ...settings, hotkey: combo });
      } catch (err) {
        setHotkeyError(String((err as Error)?.message ?? err));
      }
      stopRecording();
    };
  });
  useEffect(() => {
    if (!recording) return;
    const h = (e: KeyboardEvent) => void recordHandler.current(e);
    window.addEventListener("keydown", h, true);
    return () => window.removeEventListener("keydown", h, true);
  }, [recording]);

  /* ---------- data dir ---------- */

  // Pick a folder in the native dialog, then move data there directly
  // (pre-checks run inside SetDataDir; failures surface as a red message).
  const handleMoveData = useCallback(async () => {
    setMigrateMsg(null);
    setMoving(true);
    try {
      const dir = await chooseDataDir();
      if (!dir) return; // user cancelled — keep the panel quiet
      const err = await setDataDir(dir);
      if (err) {
        setMigrateMsg({ ok: false, text: err });
        return;
      }
      const d = await currentDataDir().catch(() => "");
      if (d) setDataDirDisplay(d);
      setMigrateMsg({ ok: true, text: t.migrateDone });
    } catch (err) {
      setMigrateMsg({ ok: false, text: String((err as Error)?.message ?? err) });
    } finally {
      setMoving(false);
    }
  }, []);

  if (!open) return null;

  // Backwards-compatible opacity: unset (0 / missing) means fully opaque.
  const opacityValue = settings.opacity && settings.opacity >= 0.3 ? settings.opacity : 1;

  const sectionLabel =
    "mb-1.5 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-[var(--fg-muted)]";
  const primaryBtn =
    "flex items-center gap-1 rounded bg-[var(--accent)] px-2.5 py-1.5 text-[11px] font-medium text-[var(--accent-fg)] hover:opacity-90 disabled:opacity-50";
  const secondaryBtn =
    "flex items-center gap-1 rounded border border-[var(--border)] bg-[var(--bg-input)] px-2.5 py-1.5 text-[11px] font-medium hover:bg-[var(--hover)]";

  return (
    <div
      className="fixed inset-0 z-[100] flex items-start justify-center bg-black/30 p-4 pt-14"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) handleClose();
      }}
    >
      {/* While recording, show a dim hint so the user knows a combo is being
          captured. */}
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
            onClick={handleClose}
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
                onClick={recording ? stopRecording : () => void beginRecording()}
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

          {/* Window opacity */}
          <section>
            <div className={sectionLabel}>
              <Droplets size={11} /> {t.opacitySection}
            </div>
            <p className="mb-2 text-[10px] leading-snug text-[var(--fg-muted)]">{t.opacityDesc}</p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                min={0.35}
                max={1}
                step={0.05}
                value={opacityValue}
                onChange={(e) => onChanged({ ...settings, opacity: Number(e.target.value) })}
                className="flex-1 accent-[var(--accent)]"
                aria-label={t.opacitySection}
              />
              <span className="w-10 text-right text-[11px] tabular-nums">
                {Math.round(opacityValue * 100)}%
              </span>
            </div>
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
            <div className="flex gap-1.5">
              <button className={secondaryBtn} onClick={() => void openDataDir()}>
                <FolderOpen size={11} /> {t.openExplorer}
              </button>
              <button className={secondaryBtn} onClick={() => void handleMoveData()} disabled={moving}>
                {moving ? <Loader2 size={11} className="animate-spin" /> : <FolderSearch size={11} />}
                {t.moveData}
              </button>
            </div>
            {migrateMsg && (
              <p
                className={`mt-2 text-[10px] ${
                  migrateMsg.ok
                    ? "text-green-600 dark:text-green-400"
                    : "text-red-500"
                }`}
              >
                {migrateMsg.ok && <Check size={11} className="mr-1 inline" />}
                {migrateMsg.text}
              </p>
            )}
          </section>
          {/* About */}
          <section>
            <div className={sectionLabel}>
              <Info size={11} /> {t.aboutSection}
            </div>
            <p className="mb-2 text-[10px] leading-snug text-[var(--fg-muted)]">{t.aboutDesc}</p>
            <div className="mb-1.5 space-y-1 text-[10px]">
              <div className="flex justify-between">
                <span className="text-[var(--fg-muted)]">{t.versionLabel}</span>
                <span className="font-semibold">v{version}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--fg-muted)]">{t.homepageLabel}</span>
                <button
                  className="font-medium text-[var(--accent)] hover:underline"
                  onClick={() => void openUrl(HOME_URL)}
                >
                  github.com/zyition/slite-note
                </button>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--fg-muted)]">{t.licenseLabel}</span>
                <button
                  className="font-medium text-[var(--accent)] hover:underline"
                  onClick={() => void openUrl(HOME_URL + "/blob/main/LICENSE")}
                >
                  MIT
                </button>
              </div>
            </div>
            <p className="text-[9px] leading-snug text-[var(--fg-muted)]">
              © 2025 zyition · Built with Wails, BlockNote, React, Tailwind &amp; lucide.
            </p>
          </section>
        </div>
      </div>
    </div>
  );
}
