import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

export const buttonVariants = cva(
  "press inline-flex shrink-0 select-none items-center justify-center gap-2 whitespace-nowrap font-medium transition-[background-color,color,border-color,transform] duration-150 disabled:pointer-events-none disabled:opacity-45",
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

export type ButtonProps = ComponentProps<"button"> & VariantProps<typeof buttonVariants>;

export function Button({ className, variant, size, type = "button", ...props }: ButtonProps) {
  return <button type={type} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
