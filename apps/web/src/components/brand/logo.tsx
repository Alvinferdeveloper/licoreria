import { cn } from "@/lib/cn";

/** The barrel mark; follows the theme's amber. */
export function LogoMark({ size = 24, className }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden className={cn("shrink-0", className)}>
      <rect width="32" height="32" rx="8" fill="var(--amber)" />
      <path d="M11.2 6h9.6c2.1 3 3.2 6.4 3.2 10s-1.1 7-3.2 10h-9.6C9.1 23 8 19.6 8 16s1.1-7 3.2-10z" fill="var(--on-accent)" />
      <path d="M8.5 11.6q7.5 1.2 15 0M8.5 20.4q7.5 1.2 15 0" fill="none" stroke="var(--amber)" strokeWidth="2.2" />
    </svg>
  );
}

const SIZES = {
  sm: { mark: 24, text: "text-[1.375rem]", gap: "gap-2" },
  lg: { mark: 48, text: "text-[2.5rem]", gap: "gap-3" },
} as const;

/** Mark and wordmark; the name is read by screen readers once. */
export function Logo({ size = "sm", className }: { size?: keyof typeof SIZES; className?: string }) {
  const s = SIZES[size];
  return (
    <span className={cn("inline-flex items-center", s.gap, className)}>
      <LogoMark size={s.mark} />
      <span className={cn("font-brand leading-none font-semibold tracking-[-0.01em] text-ink", s.text)}>Añejo</span>
    </span>
  );
}
