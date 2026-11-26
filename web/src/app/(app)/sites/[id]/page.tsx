import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Overview } from "@/components/dashboard/overview";
import { Panel } from "@/components/dashboard/panels";
import { FilterChips, RealtimeCounter, Toolbar } from "@/components/dashboard/toolbar";
import { buttonVariants } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { api, ApiRequestError } from "@/lib/api";
import { PANELS } from "@/lib/dimensions";
import { parseQuery } from "@/lib/query";
import type { Breakdown, Interval } from "@/lib/types";

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  try {
    return { title: (await api.site(id)).name };
  } catch {
    return { title: "Site" };
  }
}

/** The bucket "now" falls in, in the site's time zone, in the API's bucket format. */
function nowKey(timezone: string, interval: Interval): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date());
  const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "";
  const date = `${get("year")}-${get("month")}-${get("day")}`;
  if (interval === "hour") return `${date}T${get("hour")}:00`;
  return interval === "day" ? date : date.slice(0, 7);
}

function rangeLabel(from: string, to: string): string {
  const fmt = new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
  const end = new Date(`${to.slice(0, 10)}T00:00:00Z`);
  end.setUTCDate(end.getUTCDate() - 1); // the API's range ends at the start of the next day
  return `${fmt.format(new Date(`${from.slice(0, 10)}T00:00:00Z`))} – ${fmt.format(end)}`;
}

export default async function SiteDashboard({ params, searchParams }: Props) {
  const { id } = await params;
  const query = parseQuery(await searchParams);

  let site;
  try {
    site = await api.site(id);
  } catch (err) {
    if (err instanceof ApiRequestError && err.status === 404) notFound();
    throw err;
  }

  const dims = PANELS.flatMap((p) => p.tabs.map((t) => t.dim));
  let summary, series, realtime, breakdowns: Breakdown[];
  try {
    [summary, series, realtime, ...breakdowns] = await Promise.all([
      api.summary(id, query),
      api.timeseries(id, query),
      api.realtime(id),
      ...dims.map((d) => api.breakdown(id, query, d, 10)),
    ] as const);
  } catch (err) {
    if (err instanceof ApiRequestError && err.status === 400) {
      return (
        <Card className="mx-auto flex max-w-md flex-col items-center gap-3 p-8 text-center">
          <p className="font-medium">This view cannot be shown</p>
          <p className="text-muted-foreground text-sm">
            {err.body.message ?? "The period or filters are not valid."}
          </p>
          <Link
            href={`/sites/${id}`}
            className={buttonVariants({ variant: "secondary", size: "sm" })}
          >
            Reset the view
          </Link>
        </Card>
      );
    }
    throw err;
  }
  const byDim: Record<string, Breakdown> = {};
  dims.forEach((d, i) => {
    byDim[d] = breakdowns[i] as Breakdown;
  });

  return (
    <div className="flex flex-col gap-5">
      <Link
        href="/"
        className="text-muted-foreground hover:text-foreground flex w-fit items-center gap-1 text-sm"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Sites
      </Link>
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
        <div className="flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1">
          <h1 className="truncate text-xl font-semibold tracking-tight">{site.name}</h1>
          <span className="text-muted-foreground text-sm">{site.domain}</span>
          <RealtimeCounter siteId={site.id} initial={realtime.visitors} />
        </div>
        <Toolbar
          siteId={site.id}
          periodLabel={rangeLabel(summary.period.from, summary.period.to)}
        />
      </div>
      <FilterChips />
      <Overview summary={summary} series={series} nowKey={nowKey(site.timezone, series.interval)} />
      <div className="grid gap-5 lg:grid-cols-2">
        {PANELS.map((p) => (
          <Panel key={p.id} siteId={site.id} panel={p} data={byDim} />
        ))}
      </div>
      <p className="text-faint text-center text-xs">
        Times are in {site.timezone}. Visitors are counted per day, so a person who returns on
        another day counts again.
      </p>
    </div>
  );
}
