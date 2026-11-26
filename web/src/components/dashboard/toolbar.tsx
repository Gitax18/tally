"use client";

import { CalendarDays, ChevronDown, Settings, X } from "lucide-react";
import Link from "next/link";
import { DropdownMenu } from "radix-ui";
import * as React from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { DIMENSIONS, displayValue } from "@/lib/dimensions";
import { PERIODS, withoutFilter } from "@/lib/query";
import type { Realtime } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useDashboard } from "./nav";

const REALTIME_POLL_MS = 30_000;

/** "N current visitors": seen in the last five minutes, refreshed every 30 seconds. */
export function RealtimeCounter({ siteId, initial }: { siteId: string; initial: number }) {
  const [count, setCount] = React.useState(initial);
  React.useEffect(() => {
    let stopped = false;
    const tick = async () => {
      if (document.visibilityState === "hidden") return;
      try {
        const res = await fetch(`/api/sites/${siteId}/realtime`, { cache: "no-store" });
        if (res.ok && !stopped) setCount(((await res.json()) as Realtime).visitors);
      } catch {
        // Keep the last number; try again on the next tick.
      }
    };
    const timer = setInterval(tick, REALTIME_POLL_MS);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [siteId]);
  return (
    <p
      className="text-muted-foreground flex items-center gap-2 text-sm"
      aria-live="polite"
      data-testid="realtime"
    >
      <span
        className={cn("size-2 rounded-full", count > 0 ? "bg-ok animate-pulse-soft" : "bg-faint")}
        aria-hidden
      />
      <span className="tabular">
        <span className="text-foreground font-semibold">{count}</span> current visitor
        {count === 1 ? "" : "s"}
      </span>
    </p>
  );
}

export function Toolbar({ siteId, periodLabel }: { siteId: string; periodLabel: string }) {
  const { query, navigate, pending } = useDashboard();
  const [customOpen, setCustomOpen] = React.useState(false);
  const current = PERIODS.find((p) => p.value === query.period)?.label ?? "Last 30 days";
  return (
    <div className="flex items-center gap-2">
      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          className={cn(
            buttonVariants({ variant: "secondary", size: "sm" }),
            pending && "opacity-70",
          )}
        >
          <CalendarDays aria-hidden />
          <span>{query.period === "custom" ? periodLabel : current}</span>
          <ChevronDown aria-hidden />
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={6}
            className="bg-card border-border shadow-lift rounded-control z-50 min-w-44 border p-1"
          >
            {PERIODS.map((p) => (
              <DropdownMenu.Item
                key={p.value}
                onSelect={() =>
                  p.value === "custom"
                    ? setCustomOpen(true)
                    : navigate({ ...query, period: p.value, from: undefined, to: undefined })
                }
                className={cn(
                  "data-highlighted:bg-surface-2 cursor-pointer rounded-[5px] px-2.5 py-1.5 text-sm outline-none",
                  p.value === query.period && "text-accent font-semibold",
                )}
              >
                {p.label}
              </DropdownMenu.Item>
            ))}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
      <CustomRange open={customOpen} onOpenChange={setCustomOpen} />
      <Link
        href={`/sites/${siteId}/settings`}
        className={buttonVariants({ variant: "ghost", size: "icon" })}
        aria-label="Site settings"
      >
        <Settings aria-hidden />
      </Link>
    </div>
  );
}

function CustomRange({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { query, navigate } = useDashboard();
  const [error, setError] = React.useState<string>();
  function apply(form: FormData) {
    const from = String(form.get("from") ?? "");
    const to = String(form.get("to") ?? "");
    if (!from || !to || from > to) {
      setError("Choose a start date on or before the end date.");
      return;
    }
    onOpenChange(false);
    navigate({ ...query, period: "custom", from, to });
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        title="Custom range"
        description="Both days are included."
        className="max-w-sm"
      >
        <form action={apply} className="flex flex-col gap-4 p-5">
          <div className="grid grid-cols-2 gap-3">
            <Field label="From" htmlFor="from">
              <Input id="from" name="from" type="date" required defaultValue={query.from} />
            </Field>
            <Field label="To" htmlFor="to">
              <Input id="to" name="to" type="date" required defaultValue={query.to} />
            </Field>
          </div>
          {error ? (
            <p className="text-bad text-sm" role="alert">
              {error}
            </p>
          ) : null}
          <Button type="submit">Apply</Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** The active filters as removable chips. */
export function FilterChips() {
  const { query, navigate } = useDashboard();
  if (query.filters.length === 0) return null;
  return (
    <ul className="flex flex-wrap items-center gap-2" aria-label="Filters">
      {query.filters.map((f) => (
        <li
          key={f.dim}
          className="bg-accent-soft text-foreground rounded-control flex max-w-full items-center gap-1 py-1 pr-1 pl-2.5 text-sm"
        >
          <span className="text-muted-foreground shrink-0">
            {DIMENSIONS[f.dim]?.label ?? f.dim} is
          </span>
          <span className="truncate font-medium">{displayValue(f.dim, f.value)}</span>
          <button
            type="button"
            onClick={() => navigate(withoutFilter(query, f.dim))}
            className="text-muted-foreground hover:text-foreground hover:bg-card grid size-6 shrink-0 place-items-center rounded-[5px]"
            aria-label={`Remove the ${DIMENSIONS[f.dim]?.label ?? f.dim} filter`}
          >
            <X className="size-3.5" aria-hidden />
          </button>
        </li>
      ))}
      {query.filters.length > 1 ? (
        <li>
          <button
            type="button"
            onClick={() => navigate({ ...query, filters: [] })}
            className="text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline"
          >
            Clear all
          </button>
        </li>
      ) : null}
    </ul>
  );
}
