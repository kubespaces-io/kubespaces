import type { InputHTMLAttributes } from "react";

export interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  id: string;
  label: string;
  hint?: string;
  error?: string | null;
}

export function Field({
  id,
  label,
  hint,
  error,
  className = "",
  ...props
}: FieldProps) {
  const describedBy =
    [hint ? `${id}-hint` : null, error ? `${id}-error` : null]
      .filter(Boolean)
      .join(" ") || undefined;

  return (
    <div className="flex flex-col gap-1.5">
      <label
        htmlFor={id}
        className="text-[0.6875rem] font-medium uppercase tracking-[0.12em] text-ink-muted"
      >
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={
          "border-b bg-transparent px-0 py-1.5 font-body text-[0.9375rem] text-ink " +
          "placeholder:text-ink-faint focus:outline-none transition-colors " +
          (error
            ? "border-status-red "
            : "border-ink/25 hover:border-ink/50 focus:border-accent ") +
          className
        }
        {...props}
      />
      {hint && !error && (
        <p id={`${id}-hint`} className="text-xs leading-relaxed text-ink-faint">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="text-xs text-status-red">
          {error}
        </p>
      )}
    </div>
  );
}
