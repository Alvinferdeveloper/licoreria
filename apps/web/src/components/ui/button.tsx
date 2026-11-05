import { CircleNotch } from "@phosphor-icons/react/dist/ssr";
import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

export const buttonVariants = cva(
  "press relative inline-flex shrink-0 select-none items-center justify-center gap-2 whitespace-nowrap font-medium transition-[background-color,color,border-color,transform] duration-150 disabled:pointer-events-none disabled:opacity-45",
  {
    variants: {
      variant: {
        primary: "bg-amber text-on-accent shadow-(--shadow-edge) hover:bg-amber-hover",
        secondary: "border border-line-strong bg-surface text-ink hover:bg-chrome",
        ghost: "text-ink-muted hover:bg-sunken hover:text-ink",
        danger: "bg-danger text-on-accent hover:opacity-90",
      },
      size: {
        sm: "h-8 rounded-md px-3 text-label",
        md: "h-9 rounded-md px-4 text-label",
        lg: "h-11 rounded-md px-5 text-body",
        xl: "h-16 rounded-lg px-6 text-title tracking-headline",
        icon: "size-9 rounded-md",
        "icon-sm": "size-8 rounded-md",
      },
    },
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export type ButtonProps = ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    /** Keeps the button's width, blurs its label and shows a spinner. */
    loading?: boolean;
  };

export function Button({ className, variant, size, loading, disabled, children, type = "button", ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(buttonVariants({ variant, size }), loading && "disabled:opacity-100", className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      <span className={cn("inline-flex items-center gap-2 transition-[filter,opacity] duration-200", loading && "opacity-60 blur-[2px]")}>
        {children}
      </span>
      {loading && <CircleNotch size={16} weight="bold" className="absolute animate-spin" aria-hidden />}
    </button>
  );
}
