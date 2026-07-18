"use client";

import { useRef } from "react";
import type { ReactNode } from "react";
import { useFocusTrap } from "@/hooks/useFocusTrap";

interface DialogProps {
  open: boolean;
  onClose: () => void;
  /** id of the element labelling the dialog (usually its heading). */
  labelledBy: string;
  children: ReactNode;
}

/**
 * Minimal modal dialog: overlay, focus trap, Escape + overlay-click to close.
 */
export function Dialog({ open, onClose, labelledBy, children }: DialogProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  useFocusTrap(panelRef, open, onClose);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-ink/40 px-4 py-[12vh]"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
        className="w-full max-w-md border border-ink/15 bg-paper-raised p-8 shadow-[0_24px_64px_-24px_rgba(25,23,19,0.4)]"
      >
        {children}
      </div>
    </div>
  );
}
