"use client";

import { useEffect } from "react";
import type { RefObject } from "react";

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(", ");

/**
 * Traps Tab focus inside `containerRef` while `isActive`, closes on Escape,
 * focuses the first focusable element on open and restores focus on close.
 */
export function useFocusTrap(
  containerRef: RefObject<HTMLElement | null>,
  isActive: boolean,
  onClose: () => void,
): void {
  useEffect(() => {
    if (!isActive) return;
    const container = containerRef.current;
    if (!container) return;

    const previouslyFocused = document.activeElement as HTMLElement | null;
    const focusables = () =>
      Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
    (focusables()[0] ?? container).focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.key !== "Tab") return;

      const items = focusables();
      if (items.length === 0) {
        event.preventDefault();
        return;
      }
      const first = items[0];
      const last = items[items.length - 1];
      const active = document.activeElement;

      if (event.shiftKey && (active === first || active === container)) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown, true);
    const { overflow } = document.body.style;
    document.body.style.overflow = "hidden";

    return () => {
      document.removeEventListener("keydown", handleKeyDown, true);
      document.body.style.overflow = overflow;
      previouslyFocused?.focus();
    };
  }, [containerRef, isActive, onClose]);
}
