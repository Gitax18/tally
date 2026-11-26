"use client";

import * as React from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card } from "@/components/ui/card";
import { bucketLabel, formatChange, formatMetric } from "@/lib/format";
import { METRICS } from "@/lib/query";
import type { Interval, MetricKey, Summary, Timeseries } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useDashboard } from "./nav";

/** The six metric cards and the chart of the selected one. */
export function Overview({
  summary,
  series,
  nowKey,
}: {
  summary: Summary;
  series: Timeseries;
  /** The current bucket in the site's time zone; later buckets have no data yet. */
  nowKey: string;
}) {
  const { query, setMetric } = useDashboard();
  const metric = query.metric;
  return (
    <Card className="overflow-hidden">
      <div
        className="border-border grid grid-cols-2 border-b sm:grid-cols-3 lg:grid-cols-6"
        role="tablist"
        aria-label="Metric"
      >
        {METRICS.map((m) => {
          const change = formatChange(m.key, summary.change[m.key]);
          const selected = m.key === metric;
          return (
            <button
              key={m.key}
              type="button"
              role="tab"
              aria-selected={selected}
              onClick={() => setMetric(m.key)}
              data-testid={`metric-${m.key}`}
              className={cn(
                "border-border relative flex flex-col items-start gap-1 border-r border-b px-4 py-3.5 text-left transition-colors last:border-r-0 lg:border-b-0",
                "hover:bg-surface-2 focus-visible:outline-accent focus-visible:-outline-offset-2 focus-visible:outline-2",
                selected && "bg-surface-2",
              )}
            >
              {selected ? (
                <span className="bg-accent absolute inset-x-0 bottom-0 h-0.5" aria-hidden />
              ) : null}
              <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
                {m.label}
              </span>
              <span className="flex items-baseline gap-2">
                <span className="tabular text-xl font-semibold" data-testid={`value-${m.key}`}>
                  {formatMetric(m.key, summary.current[m.key])}
                </span>
                {change ? (
                  <span
                    className={cn(
                      "tabular text-xs font-semibold",
                      change.tone === "good" && "text-ok",
                      change.tone === "bad" && "text-bad",
                      change.tone === "neutral" && "text-muted-foreground",
                    )}
                    title="Compared with the previous period"
                  >
                    {change.text}
                  </span>
                ) : null}
              </span>
            </button>
          );
        })}
      </div>
      <div className="h-72 px-2 pt-4 pb-2 sm:px-4">
        <Chart series={series} metric={metric} nowKey={nowKey} />
      </div>
    </Card>
  );
}

type Row = {
  bucket: string;
  current: number | null;
  previous: number | null;
  previousBucket: string | null;
};

function Chart({
  series,
  metric,
  nowKey,
}: {
  series: Timeseries;
  metric: MetricKey;
  nowKey: string;
}) {
  const data: Row[] = series.current.map((p, i) => {
    const prev = series.previous[i];
    return {
      bucket: p.bucket,
      // Buckets after now are not zero, they just have not happened: leave them out of the line.
      current: p.bucket > nowKey ? null : p[metric],
      previous: prev ? prev[metric] : null,
      previousBucket: prev?.bucket ?? null,
    };
  });
  const interval = series.interval;
  const label = METRICS.find((m) => m.key === metric)?.label ?? metric;
  return (
    <ResponsiveContainer width="100%" height="100%">
      <ComposedChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
        <CartesianGrid vertical={false} stroke="var(--chart-grid)" />
        <XAxis
          dataKey="bucket"
          tickFormatter={(b: string) => bucketLabel(b, interval)}
          tick={{ fill: "var(--muted-foreground)", fontSize: 12 }}
          tickLine={false}
          axisLine={false}
          minTickGap={24}
        />
        <YAxis
          width={48}
          tickFormatter={(v: number) => formatMetric(metric, v)}
          tick={{ fill: "var(--muted-foreground)", fontSize: 12 }}
          tickLine={false}
          axisLine={false}
          allowDecimals={metric === "views_per_visit"}
        />
        <Tooltip
          cursor={{ stroke: "var(--border-strong)" }}
          content={(props) => (
            <ChartTooltip
              active={props.active}
              payload={props.payload}
              metric={metric}
              label={label}
              interval={interval}
            />
          )}
        />
        <Line
          dataKey="previous"
          type="monotone"
          stroke="var(--chart-previous)"
          strokeWidth={1.5}
          strokeDasharray="4 4"
          dot={false}
          activeDot={false}
          isAnimationActive={false}
        />
        <Area
          dataKey="current"
          type="monotone"
          stroke="var(--chart-line)"
          strokeWidth={2}
          fill="var(--chart-fill)"
          dot={false}
          activeDot={{ r: 3.5, strokeWidth: 0, fill: "var(--chart-line)" }}
          isAnimationActive={false}
          connectNulls={false}
        />
      </ComposedChart>
    </ResponsiveContainer>
  );
}

function ChartTooltip({
  active,
  payload,
  metric,
  label,
  interval,
}: {
  active?: boolean | undefined;
  payload?: ReadonlyArray<{ payload?: unknown }> | undefined;
  metric: MetricKey;
  label: string;
  interval: Interval;
}) {
  const row = payload?.[0]?.payload as Row | undefined;
  if (!active || !row) return null;
  return (
    <div className="bg-card border-border shadow-lift rounded-control border px-3 py-2 text-sm">
      <p className="text-muted-foreground mb-1 text-xs font-medium">{label}</p>
      <p className="flex items-center justify-between gap-6">
        <span className="flex items-center gap-2">
          <span className="bg-accent size-2 rounded-full" aria-hidden />
          {bucketLabel(row.bucket, interval, true)}
        </span>
        <span className="tabular font-semibold">
          {row.current === null ? "—" : formatMetric(metric, row.current)}
        </span>
      </p>
      {row.previousBucket !== null && row.previous !== null ? (
        <p className="text-muted-foreground flex items-center justify-between gap-6">
          <span className="flex items-center gap-2">
            <span className="border-faint size-2 rounded-full border border-dashed" aria-hidden />
            {bucketLabel(row.previousBucket, interval, true)}
          </span>
          <span className="tabular">{formatMetric(metric, row.previous)}</span>
        </p>
      ) : null}
    </div>
  );
}
