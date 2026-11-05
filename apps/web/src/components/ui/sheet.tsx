"use client";

import { X } from "@phosphor-icons/react";
import { useEffect, useRef, type ReactNode } from "react";
import { Button } from "./button";

interface SheetProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
}

/** Side panel on a native <dialog>: focus trap, Esc to close and inert background come built in. */
export function Sheet({ open, onClose, title, description, children, footer }: SheetProps) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      onClose={onClose}
      onClick={(event) => event.target === ref.current && onClose()}
      aria-labelledby="sheet-title"
      className="sheet fixed inset-y-0 right-0 left-auto m-0 h-dvh max-h-dvh w-full max-w-md bg-surface p-0 text-ink shadow-(--shadow-float-high) backdrop:bg-[oklch(18.5%_0.01_60/0.4)]"
    >
      {open && (
        <div className="flex h-full flex-col">
          <header className="flex items-start justify-between gap-4 border-b border-line px-6 py-5">
            <div>
              <h2 id="sheet-title" className="text-title font-semibold tracking-headline">
                {title}
              </h2>
              {description && <p className="mt-1 text-label text-ink-muted">{description}</p>}
            </div>
            <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Cerrar">
              <X size={18} />
            </Button>
          </header>
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">{children}</div>
          {footer && <footer className="flex justify-end gap-2 border-t border-line px-6 py-4">{footer}</footer>}
        </div>
      )}
    </dialog>
  );
}
