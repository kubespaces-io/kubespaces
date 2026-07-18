const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** Kubernetes-style compact age, e.g. "38s", "12m", "3h", "5d". */
export function formatAge(iso: string, now: Date = new Date()): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const seconds = Math.max(0, Math.floor((now.getTime() - then) / 1000));
  if (seconds < MINUTE) return `${seconds}s`;
  if (seconds < HOUR) return `${Math.floor(seconds / MINUTE)}m`;
  if (seconds < DAY) return `${Math.floor(seconds / HOUR)}h`;
  return `${Math.floor(seconds / DAY)}d`;
}

/** Full local timestamp for detail views. */
export function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
