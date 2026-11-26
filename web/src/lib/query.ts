// The dashboard's state lives in the URL: ?period=7d&f=source:GitHub&f=country:IN&metric=visits
// (plus from/to for a custom range). Bookmarks and the back button just work.
import type { MetricKey } from "./types";

export type Filter = { dim: string; value: string };

export type DashboardQuery = {
  period: string;
  from?: string;
  to?: string;
  filters: Filter[];
  metric: MetricKey;
};

export const PERIODS: { value: string; label: string }[] = [
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "month", label: "This month" },
  { value: "last_month", label: "Last month" },
  { value: "12mo", label: "Last 12 months" },
  { value: "custom", label: "Custom range" },
];

export const METRICS: { key: MetricKey; label: string }[] = [
  { key: "visitors", label: "Unique visitors" },
  { key: "visits", label: "Visits" },
  { key: "pageviews", label: "Pageviews" },
  { key: "views_per_visit", label: "Views per visit" },
  { key: "bounce_rate", label: "Bounce rate" },
  { key: "visit_duration", label: "Visit duration" },
];

// Must match internal/filter in the Go service.
export const FILTER_DIMENSIONS = [
  "source",
  "referrer",
  "utm_source",
  "utm_medium",
  "utm_campaign",
  "page",
  "entry_page",
  "exit_page",
  "country",
  "browser",
  "os",
  "device",
  "hostname",
] as const;

const DEFAULT_PERIOD = "30d";
const DEFAULT_METRIC: MetricKey = "visitors";
const DATE = /^\d{4}-\d{2}-\d{2}$/;

type Params = URLSearchParams | Record<string, string | string[] | undefined>;

function all(params: Params, key: string): string[] {
  if (params instanceof URLSearchParams) return params.getAll(key);
  const v = params[key];
  if (v === undefined) return [];
  return Array.isArray(v) ? v : [v];
}

function one(params: Params, key: string): string | undefined {
  return all(params, key)[0];
}

/** Reads the dashboard state, ignoring anything malformed rather than failing. */
export function parseQuery(params: Params): DashboardQuery {
  let period = one(params, "period") ?? DEFAULT_PERIOD;
  if (!PERIODS.some((p) => p.value === period)) period = DEFAULT_PERIOD;
  const from = one(params, "from");
  const to = one(params, "to");
  const q: DashboardQuery = { period, filters: [], metric: DEFAULT_METRIC };
  if (period === "custom") {
    if (from && to && DATE.test(from) && DATE.test(to)) {
      q.from = from;
      q.to = to;
    } else {
      q.period = DEFAULT_PERIOD;
    }
  }
  const seen = new Set<string>();
  for (const raw of all(params, "f")) {
    const i = raw.indexOf(":");
    if (i <= 0) continue;
    const dim = raw.slice(0, i);
    if (!(FILTER_DIMENSIONS as readonly string[]).includes(dim) || seen.has(dim)) continue;
    seen.add(dim);
    q.filters.push({ dim, value: raw.slice(i + 1) });
  }
  const metric = one(params, "metric");
  if (metric && METRICS.some((m) => m.key === metric)) q.metric = metric as MetricKey;
  return q;
}

/** The URL search string for a state, leaving defaults out. */
export function toSearch(q: DashboardQuery): string {
  const p = new URLSearchParams();
  if (q.period !== DEFAULT_PERIOD) p.set("period", q.period);
  if (q.period === "custom" && q.from && q.to) {
    p.set("from", q.from);
    p.set("to", q.to);
  }
  for (const f of q.filters) p.append("f", `${f.dim}:${f.value}`);
  if (q.metric !== DEFAULT_METRIC) p.set("metric", q.metric);
  const s = p.toString();
  return s ? `?${s}` : "";
}

/** The Go API's filters parameter: dim:value pairs with each value percent-encoded. */
export function apiFilters(filters: Filter[]): string {
  return filters.map((f) => `${f.dim}:${encodeURIComponent(f.value)}`).join(",");
}

/** The Go API's period parameters for a state. */
export function apiPeriodParams(q: DashboardQuery): URLSearchParams {
  const p = new URLSearchParams({ period: q.period });
  if (q.period === "custom" && q.from && q.to) {
    p.set("from", q.from);
    p.set("to", q.to);
  }
  if (q.filters.length > 0) p.set("filters", apiFilters(q.filters));
  return p;
}

/** Adds a filter, replacing any existing filter on the same dimension. */
export function withFilter(q: DashboardQuery, f: Filter): DashboardQuery {
  return { ...q, filters: [...q.filters.filter((x) => x.dim !== f.dim), f] };
}

export function withoutFilter(q: DashboardQuery, dim: string): DashboardQuery {
  return { ...q, filters: q.filters.filter((x) => x.dim !== dim) };
}
