import { Minus, Palette, Pin, PinOff, Plus, Settings as SettingsIcon, X } from "lucide-react";
import type { Note } from "../types/note";
import { t } from "../services/i18n";
import { NotePicker } from "./NotePicker";

interface TitleBarProps {
  notes: Note[];
  activeId: string | null;
  titleFor: (note: Note) => string;
  pinned: boolean;
  onSelect: (id: string) => void;
  onNewNote: () => void;
  onCycleTheme: () => void;
  onTogglePin: () => void;
  onDeleteNote: (id: string) => void;
  onOpenSettings: () => void;
  onHide: () => void;
  onClose: () => void;
}

/**
 * TitleBar — the custom frameless window chrome. The whole bar is a Wails
 * drag region (CSS --wails-draggable: drag); interactive children opt out
 * with the .no-drag class.
 */
export function TitleBar(props: TitleBarProps) {
  const {
    notes,
    activeId,
    titleFor,
    pinned,
    onSelect,
    onNewNote,
    onCycleTheme,
    onTogglePin,
    onDeleteNote,
    onOpenSettings,
    onHide,
    onClose,
  } = props;

  const iconBtn =
    "no-drag flex h-6 w-6 items-center justify-center rounded text-[var(--fg-muted)] hover:bg-[var(--hover)] hover:text-[var(--fg)]";

  return (
    <header className="titlebar flex h-10 shrink-0 items-center gap-0.5 border-b border-[var(--border)] bg-[var(--bg-titlebar)] px-2">
      <NotePicker
        notes={notes}
        activeId={activeId}
        titleFor={titleFor}
        onSelect={onSelect}
        onDeleteNote={onDeleteNote}
      />

      {/* Empty drag zone: everything between the picker and the buttons drags the window. */}
      <div className="min-w-2 flex-1 self-stretch" aria-hidden="true" />

      <button className={iconBtn} onClick={onOpenSettings} title={t.settings}>
        <SettingsIcon size={13} />
      </button>

      <button className={iconBtn} onClick={onTogglePin} title={t.alwaysOnTop} aria-pressed={pinned}>
        {pinned ? (
          <Pin size={13} className="text-[var(--accent)]" fill="currentColor" />
        ) : (
          <PinOff size={13} />
        )}
      </button>

      <button className={iconBtn} onClick={onCycleTheme} title={t.theme}>
        <Palette size={13} />
      </button>

      <button className={iconBtn} onClick={onNewNote} title={t.newNote}>
        <Plus size={14} />
      </button>

      <button className={iconBtn} onClick={onHide} title={t.hide}>
        <Minus size={13} />
      </button>

      <button className={`${iconBtn} hover:bg-red-500/80 hover:text-white`} onClick={onClose} title={t.close}>
        <X size={13} />
      </button>
    </header>
  );
}
