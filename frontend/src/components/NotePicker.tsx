import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown, Trash2, X } from "lucide-react";
import type { Note } from "../types/note";
import { t } from "../services/i18n";

interface NotePickerProps {
  notes: Note[];
  activeId: string | null;
  titleFor: (note: Note) => string;
  onSelect: (id: string) => void;
  onDeleteNote: (id: string) => void;
}

/**
 * NotePicker — the collapsed note list in the title bar (per the "top
 * dropdown" UX decision). Click to open, click a row to switch, hover a row
 * to reveal the delete action (with an inline confirm), click outside or
 * press Escape to dismiss.
 */
export function NotePicker({ notes, activeId, titleFor, onSelect, onDeleteNote }: NotePickerProps) {
  const [open, setOpen] = useState(false);
  const [confirmingId, setConfirmingId] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement | null>(null);

  const active = notes.find((n) => n.id === activeId) ?? null;

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        setConfirmingId(null);
      }
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const handleDelete = (id: string) => {
    onDeleteNote(id);
    setConfirmingId(null);
  };

  return (
    <div ref={rootRef} className="no-drag relative max-w-[55%]">
      <button
        className="flex h-6 w-full max-w-52 items-center gap-1 rounded px-1.5 text-left text-[11px] font-medium hover:bg-[var(--hover)]"
        onClick={() => setOpen((v) => !v)}
        title={t.notes}
      >
        <span className="min-w-0 flex-1 truncate">
          {active ? titleFor(active) || t.untitled : t.untitled}
        </span>
        <ChevronDown size={12} className="shrink-0 text-[var(--fg-muted)]" />
      </button>

      {open && (
        <div className="absolute left-0 top-7 z-50 max-h-56 w-56 overflow-y-auto rounded-md border border-[var(--border)] bg-[var(--bg)] p-1 shadow-lg">
          {notes.length === 0 && (
            <div className="px-2 py-1.5 text-[11px] text-[var(--fg-muted)]">{t.untitled}</div>
          )}
          {notes.map((note) => {
            const isActive = note.id === activeId;
            const isConfirming = confirmingId === note.id;
            return (
              <div
                key={note.id}
                className={`group flex w-full items-center rounded px-1 text-left text-[11px] ${
                  isActive ? "bg-[var(--accent)] font-medium text-[var(--accent-fg)]" : "hover:bg-[var(--hover)]"
                }`}
              >
                <button
                  className="min-w-0 flex-1 truncate px-1.5 py-1.5"
                  onClick={() => {
                    onSelect(note.id);
                    setOpen(false);
                  }}
                >
                  {titleFor(note) || t.untitled}
                </button>
                {isConfirming ? (
                  <span className="flex shrink-0 items-center gap-0.5">
                    <span className="px-0.5 text-[10px] text-[var(--fg-muted)]">{t.deleteConfirm}</span>
                    <button
                      className="rounded p-0.5 text-[var(--fg)] hover:bg-black/10 dark:hover:bg-white/10"
                      onClick={() => handleDelete(note.id)}
                      title={t.deleteNote}
                    >
                      <Check size={12} />
                    </button>
                    <button
                      className="rounded p-0.5 text-[var(--fg)] hover:bg-black/10 dark:hover:bg-white/10"
                      onClick={() => setConfirmingId(null)}
                      title={t.cancel}
                    >
                      <X size={12} />
                    </button>
                  </span>
                ) : (
                  <button
                    className="shrink-0 rounded p-1 text-[var(--fg-muted)] opacity-0 hover:bg-black/10 hover:text-[var(--fg)] group-hover:opacity-100 dark:hover:bg-white/10"
                    onClick={() => setConfirmingId(note.id)}
                    title={t.deleteNote}
                  >
                    <Trash2 size={11} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
