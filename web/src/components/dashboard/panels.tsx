"use client";

import { Loader2, Maximize2, Search } from "lucide-react";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { DIMENSIONS, displayValue, type Column, type Panel as PanelDef } from "@/lib/dimensions";
import { formatDuration, formatNumber } from "@/lib/format";
import { toSearch, withFilter } from "@/lib/query";
import type { Breakdown, BreakdownRow } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useDashboard } from "./nav";

function cell(col: Column, row: BreakdownRow): string {
  switch (col) {
    case "time_on_page_ms":
      return row.time_on_page_ms ? formatDuration(row.time_on_page_ms / 1000) : "—";
    default:
      return formatNumber(row[col] ?? 0);
  }
}

/** One panel under the chart: tabs of breakdowns, the top 10 of each, and a details modal. */
export function Panel({
  siteId,
  panel,
  data,
}: {
  siteId: string;
  panel: PanelDef;
  data: Record<string, Breakdown>;
}) {
  const [tab, setTab] = React.useState(panel.tabs[0]!.dim);
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const breakdown = data[tab];
  const info = DIMENSIONS[tab]!;
  return (
    <Card className="flex min-h-96 flex-col" data-testid={`panel-${panel.id}`}>
      <div className="flex items-center justify-between gap-2 px-5 pt-4">
        <h2 className="font-semibold">{panel.title}</h2>
        {panel.tabs.length > 1 ? (
          <div
            className="flex flex-wrap justify-end gap-x-3 gap-y-1"
            role="tablist"
            aria-label={panel.title}
          >
            {panel.tabs.map((t) => (
              <button
                key={t.dim}
                type="button"
                role="tab"
                aria-selected={t.dim === tab}
                onClick={() => setTab(t.dim)}
                className={cn(
                  "text-[13px] font-medium underline-offset-[6px]",
                  t.dim === tab
                    ? "text-foreground decoration-accent underline decoration-2"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {t.label}
              </button>
            ))}
          </div>
        ) : null}
      </div>
      <div className="flex flex-1 flex-col px-5 pt-3 pb-4">
        <BreakdownTable
          dim={tab}
          rows={breakdown?.rows ?? []}
          columns={info.columns}
          headers={info.headers}
        />
        {breakdown && (breakdown.more || breakdown.rows.length >= 10) ? (
          <button
            type="button"
            onClick={() => setDetailsOpen(true)}
            className="text-muted-foreground hover:text-foreground mt-auto flex items-center gap-1.5 self-center pt-3 text-[13px] font-medium"
          >
            <Maximize2 className="size-3.5" aria-hidden />
            Details
          </button>
        ) : null}
      </div>
      {detailsOpen ? (
        <DetailsDialog siteId={siteId} dim={tab} open={detailsOpen} onOpenChange={setDetailsOpen} />
      ) : null}
    </Card>
  );
}

function BreakdownTable({
  dim,
  rows,
  columns,
  headers,
  onPick,
}: {
  dim: string;
  rows: BreakdownRow[];
  columns: Column[];
  headers: string[];
  onPick?: () => void;
}) {
  const { query, navigate } = useDashboard();
  const max = Math.max(1, ...rows.map((r) => r.visitors));
  const info = DIMENSIONS[dim]!;
  if (rows.length === 0) {
    return (
      <p className="text-muted-foreground grid flex-1 place-items-center py-10 text-sm">
        No data for this period
      </p>
    );
  }
  const filtered = query.filters.some((f) => f.dim === dim);
  return (
    <table className="w-full table-fixed text-sm">
      <thead>
        <tr className="text-muted-foreground text-xs">
          <th className="pb-2 text-left font-medium">{info.label}</th>
          {headers.map((h) => (
            <th key={h} className="w-16 pb-2 text-right font-medium sm:w-20">
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.value} className="group">
            <td className="py-0.5 pr-2">
              <button
                type="button"
                disabled={filtered}
                onClick={() => {
                  onPick?.();
                  navigate(withFilter(query, { dim, value: row.value }));
                }}
                className="relative flex h-8 w-full items-center overflow-hidden rounded-[5px] px-2 text-left enabled:cursor-pointer"
                title={filtered ? undefined : `Filter by ${displayValue(dim, row.value)}`}
              >
                <span
                  className="bg-accent-soft absolute inset-y-0 left-0 rounded-[5px] transition-[width]"
                  style={{ width: `${(row.visitors / max) * 100}%` }}
                  aria-hidden
                />
                <span className="relative truncate group-hover:underline group-hover:underline-offset-2">
                  {displayValue(dim, row.value)}
                </span>
              </button>
            </td>
            {columns.map((col) => (
              <td key={col} className="tabular py-0.5 text-right">
                {cell(col, row)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** The full list for one dimension, with search. */
function DetailsDialog({
  siteId,
  dim,
  open,
  onOpenChange,
}: {
  siteId: string;
  dim: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { query } = useDashboard();
  const info = DIMENSIONS[dim]!;
  const [search, setSearch] = React.useState("");
  const [result, setResult] = React.useState<{
    key: string;
    data: Breakdown | null;
    error: boolean;
  }>();
  const key = `${toSearch(query)}|${dim}|${search}`;

  React.useEffect(() => {
    const ctrl = new AbortController();
    const timer = setTimeout(async () => {
      const params = new URLSearchParams(toSearch(query).slice(1));
      params.set("dimension", dim);
      params.set("limit", "300");
      if (search.trim()) params.set("search", search.trim());
      try {
        const res = await fetch(`/api/sites/${siteId}/breakdown?${params}`, {
          signal: ctrl.signal,
          cache: "no-store",
        });
        setResult({ key, data: res.ok ? ((await res.json()) as Breakdown) : null, error: !res.ok });
      } catch {
        if (!ctrl.signal.aborted) setResult({ key, data: null, error: true });
      }
    }, 200);
    return () => {
      ctrl.abort();
      clearTimeout(timer);
    };
  }, [siteId, dim, search, query, key]);

  const loading = result?.key !== key;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title={info.label} description="Click a row to filter the dashboard by it.">
        <div className="border-border border-b px-5 py-3">
          <div className="relative">
            <Search
              className="text-faint pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
              aria-hidden
            />
            <Input
              autoFocus
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={`Search ${info.label.toLowerCase()}s`}
              className="pl-9"
              aria-label="Search"
            />
          </div>
        </div>
        <div className="min-h-48 overflow-y-auto px-5 py-3">
          {result?.error ? (
            <p className="text-bad py-8 text-center text-sm">Could not load this list.</p>
          ) : result?.data ? (
            <div className={cn(loading && "opacity-60")}>
              <BreakdownTable
                dim={dim}
                rows={result.data.rows}
                columns={info.columns}
                headers={info.headers}
                onPick={() => onOpenChange(false)}
              />
              {result.data.more ? (
                <p className="text-muted-foreground pt-3 text-center text-xs">
                  Showing the top 300. Search to narrow down.
                </p>
              ) : null}
            </div>
          ) : (
            <p className="text-muted-foreground flex justify-center py-8">
              <Loader2 className="size-5 animate-spin" aria-label="Loading" />
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
