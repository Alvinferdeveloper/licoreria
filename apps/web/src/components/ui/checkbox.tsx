import { Check } from "@phosphor-icons/react/dist/ssr";
import type { ComponentProps, ReactNode } from "react";
import { cn } from "@/lib/cn";

type CheckboxProps = Omit<ComponentProps<"input">, "type"> & { label: ReactNode };

export function Checkbox({ label, className, ...props }: CheckboxProps) {
  return (
    <label className={cn("group flex cursor-pointer select-none items-center gap-2.5 text-label", className)}>
      <span className="relative grid size-5 shrink-0 place-items-center">
        <input
          type="checkbox"
          className="peer size-5 cursor-pointer appearance-none rounded-xs border border-field bg-surface transition-colors duration-150 checked:border-amber checked:bg-amber"
          {...props}
        />
        <Check
          weight="bold"
          aria-hidden
          className="pointer-events-none absolute size-3.5 text-on-accent opacity-0 peer-checked:opacity-100"
        />
      </span>
      <span>{label}</span>
    </label>
  );
}
