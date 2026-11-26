import * as React from "react";
import { cn } from "@/lib/utils";

const fieldBase = [
  "w-full rounded-control border border-border bg-card px-3 text-sm text-card-foreground",
  "placeholder:text-faint transition-[border-color,box-shadow] duration-200",
  "hover:border-border-strong",
  "focus-visible:border-accent focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-accent/15",
  "aria-invalid:border-bad aria-invalid:focus-visible:ring-bad/15",
  "disabled:cursor-not-allowed disabled:opacity-50",
];

export function Input({ className, ...props }: React.ComponentProps<"input">) {
  return <input className={cn(fieldBase, "h-10", className)} {...props} />;
}

export function Select({ className, ...props }: React.ComponentProps<"select">) {
  return <select className={cn(fieldBase, "h-10 pr-8", className)} {...props} />;
}

/** A labelled form field with an optional hint and error. */
export function Field({
  label,
  htmlFor,
  hint,
  error,
  children,
}: {
  label: string;
  htmlFor: string;
  hint?: React.ReactNode;
  error?: string | undefined;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={htmlFor} className="text-sm font-medium">
        {label}
      </label>
      {children}
      {error ? (
        <p className="text-bad text-[13px]" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p className="text-muted-foreground text-[13px]">{hint}</p>
      ) : null}
    </div>
  );
}
