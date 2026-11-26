// Shapes of the Go API's responses (internal/api, internal/stats, internal/sites).

export type Site = {
  id: string;
  name: string;
  domain: string;
  timezone: string;
  created_at: string;
  last_event_at: string | null;
};

export type SiteWithStats = Site & {
  visitors_24h: number;
  sparkline: number[];
};

export type Metrics = {
  visitors: number;
  visits: number;
  pageviews: number;
  views_per_visit: number;
  bounce_rate: number;
  visit_duration: number;
};

export type MetricKey = keyof Metrics;

export type Change = { [K in MetricKey]: number | null };

export type PeriodInfo = {
  name: string;
  from: string;
  to: string;
  previous_from: string;
  previous_to: string;
  timezone: string;
  interval: Interval;
};

export type Interval = "hour" | "day" | "month";

export type Summary = {
  period: PeriodInfo;
  filters: string;
  current: Metrics;
  previous: Metrics;
  change: Change;
};

export type Point = Metrics & { bucket: string };

export type Timeseries = {
  period: PeriodInfo;
  interval: Interval;
  current: Point[];
  previous: Point[];
};

export type BreakdownRow = {
  value: string;
  visitors: number;
  pageviews?: number;
  visits?: number;
  time_on_page_ms?: number;
};

export type Breakdown = {
  dimension: string;
  rows: BreakdownRow[];
  more: boolean;
};

export type Realtime = { visitors: number; last_event_at: string | null };

export type Usage = {
  database_bytes: number;
  limit_bytes: number;
  percent: number;
  warn: boolean;
  events_estimate: number;
  ingest: Record<string, number>;
};

export type ApiError = { error: string; message?: string; field?: string };

/** Zones Postgres and the Go service both understand, and legacy names browsers may report. */
export type Timezones = { timezones: string[]; aliases: Record<string, string> };
