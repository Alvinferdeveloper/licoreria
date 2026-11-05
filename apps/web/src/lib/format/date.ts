const relative = new Intl.RelativeTimeFormat("es-NI", { numeric: "auto" });
const absolute = new Intl.DateTimeFormat("es-NI", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "America/Managua",
});

const UNITS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ["day", 86_400],
  ["hour", 3_600],
  ["minute", 60],
];

/** "hace 5 minutos", "ayer"; dates older than a week fall back to DD/MM/AAAA HH:MM. */
export function formatRelative(iso: string, now = Date.now()): string {
  const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
  if (Math.abs(seconds) < 60) return "hace un momento";
  if (Math.abs(seconds) > 7 * 86_400) return absolute.format(new Date(iso));
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) return relative.format(Math.trunc(seconds / size), unit);
  }
  return absolute.format(new Date(iso));
}

export const formatDateTime = (iso: string) => absolute.format(new Date(iso));
