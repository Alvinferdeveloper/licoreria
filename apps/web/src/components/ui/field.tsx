"use client";

import { Eye, EyeSlash } from "@phosphor-icons/react";
import { cloneElement, useId, useState, type ComponentProps, type ReactElement, type ReactNode } from "react";
import { cn } from "@/lib/cn";

export const inputClassName = cn(
  "h-10 w-full rounded-md border border-field bg-surface px-3 text-body text-ink",
  "transition-[border-color,box-shadow] duration-150 placeholder:text-ink-subtle",
  "focus:border-amber focus:shadow-[0_0_0_3px_color-mix(in_oklch,var(--amber)_25%,transparent)] focus:outline-none",
  "disabled:border-transparent disabled:bg-sunken disabled:text-ink-subtle",
  "aria-invalid:border-danger aria-invalid:focus:shadow-[0_0_0_3px_color-mix(in_oklch,var(--danger)_20%,transparent)]",
);

export function Input({ className, ...props }: ComponentProps<"input">) {
  return <input className={cn(inputClassName, className)} {...props} />;
}

type ControlProps = {
  id?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
};

interface FieldProps {
  label: ReactNode;
  error?: string;
  hint?: ReactNode;
  /** Optional action aligned with the label, e.g. a "forgot password" link. */
  aside?: ReactNode;
  children: ReactElement<ControlProps>;
  className?: string;
}

/** Label above the control, hint and error below, wired up for screen readers. */
export function Field({ label, error, hint, aside, children, className }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;

  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-baseline justify-between gap-3">
        <label htmlFor={id} className="text-label font-medium text-ink">
          {label}
        </label>
        {aside}
      </div>
      {cloneElement(children, {
        id,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": [hintId, errorId].filter(Boolean).join(" ") || undefined,
      })}
      {hint && !error && (
        <p id={hintId} className="text-caption text-ink-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-caption font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

export function PasswordInput({ className, ...props }: Omit<ComponentProps<"input">, "type">) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="relative">
      <input type={visible ? "text" : "password"} className={cn(inputClassName, "pr-11", className)} {...props} />
      <button
        type="button"
        onClick={() => setVisible((v) => !v)}
        aria-label={visible ? "Ocultar contraseña" : "Mostrar contraseña"}
        aria-pressed={visible}
        className="absolute top-1/2 right-1 grid size-8 -translate-y-1/2 place-items-center rounded-sm text-ink-subtle transition-colors hover:text-ink"
      >
        {visible ? <EyeSlash size={18} /> : <Eye size={18} />}
      </button>
    </div>
  );
}
