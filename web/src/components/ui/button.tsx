import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";
import * as React from "react";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center gap-2 rounded-control text-[13px] font-semibold whitespace-nowrap",
    "transition-[background-color,border-color,color,opacity] duration-200",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
    "disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
  ],
  {
    variants: {
      variant: {
        primary: "border border-transparent bg-foreground text-background hover:opacity-90",
        accent: "border border-transparent bg-accent text-accent-foreground hover:bg-accent-hover",
        secondary:
          "border border-border bg-card text-card-foreground hover:border-border-strong hover:bg-surface-2",
        danger: "border border-bad/60 bg-transparent text-bad hover:bg-bad/10",
        ghost:
          "border border-transparent text-muted-foreground hover:bg-surface-2 hover:text-foreground",
      },
      size: {
        sm: "h-8 px-3",
        md: "h-10 px-4",
        icon: "size-8 px-0",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type ButtonProps = React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & { busy?: boolean };

export function Button({
  className,
  variant,
  size,
  busy = false,
  type = "button",
  disabled,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    >
      {busy ? <Loader2 className="animate-spin" aria-hidden /> : null}
      {children}
    </button>
  );
}
