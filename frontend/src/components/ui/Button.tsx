import type { ButtonHTMLAttributes } from "react";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

const BASE =
  "inline-flex items-center justify-center gap-2 font-display text-sm font-medium tracking-tight " +
  "px-4 py-2 transition-colors duration-150 select-none " +
  "disabled:opacity-40 disabled:cursor-not-allowed " +
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

const VARIANTS: Record<ButtonVariant, string> = {
  primary:
    "bg-accent text-white hover:bg-accent-hover active:translate-y-px " +
    "disabled:hover:bg-accent",
  secondary:
    "border border-ink/25 text-ink hover:border-ink hover:bg-ink hover:text-paper " +
    "active:translate-y-px disabled:hover:border-ink/25 disabled:hover:bg-transparent disabled:hover:text-ink",
  danger:
    "bg-status-red text-white hover:bg-[#8c1c1c] active:translate-y-px disabled:hover:bg-status-red",
  ghost:
    "text-ink-muted hover:text-ink underline-offset-4 hover:underline px-2",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
}

export function Button({
  variant = "primary",
  className = "",
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={`${BASE} ${VARIANTS[variant]} ${className}`}
      {...props}
    />
  );
}
