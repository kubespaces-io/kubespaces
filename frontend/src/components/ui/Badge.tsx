import type { ReactNode } from "react";

export type BadgeTone = "amber" | "green" | "red" | "gray" | "neutral";

const TONES: Record<BadgeTone, string> = {
  amber: "text-status-amber bg-status-amber-bg",
  green: "text-status-green bg-status-green-bg",
  red: "text-status-red bg-status-red-bg",
  gray: "text-status-gray bg-status-gray-bg",
  neutral: "text-ink-muted bg-ink/5",
};

const DOTS: Record<BadgeTone, string> = {
  amber: "bg-status-amber",
  green: "bg-status-green",
  red: "bg-status-red",
  gray: "bg-status-gray",
  neutral: "bg-ink-muted",
};

interface BadgeProps {
  tone: BadgeTone;
  children: ReactNode;
  /** Animate the dot for in-progress states. */
  pulse?: boolean;
}

export function Badge({ tone, children, pulse = false }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 text-[0.6875rem] font-medium uppercase tracking-[0.08em] ${TONES[tone]}`}
    >
      <span
        aria-hidden="true"
        className={`size-1.5 rounded-full ${DOTS[tone]} ${pulse ? "animate-pulse" : ""}`}
      />
      {children}
    </span>
  );
}
