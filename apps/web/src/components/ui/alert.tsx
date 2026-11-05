import { CheckCircle, Info, WarningCircle } from "@phosphor-icons/react/dist/ssr";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

const tones = {
  danger: { className: "bg-danger-tint text-danger", Icon: WarningCircle },
  success: { className: "bg-success-tint text-success", Icon: CheckCircle },
  info: { className: "bg-info-tint text-info", Icon: Info },
} as const;

interface AlertProps {
  tone?: keyof typeof tones;
  children: ReactNode;
  className?: string;
}

/** Inline status message; color always comes with an icon and text. */
export function Alert({ tone = "danger", children, className }: AlertProps) {
  const { className: toneClass, Icon } = tones[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cn("flex items-start gap-2.5 rounded-md px-3 py-2.5 text-label", toneClass, className)}
    >
      <Icon size={18} weight="bold" className="mt-px shrink-0" aria-hidden />
      <div className="min-w-0 text-ink">{children}</div>
    </div>
  );
}
