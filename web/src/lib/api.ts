// The Go API client. It runs only on the server: it carries API_SECRET.
import "server-only";
import { loadEnv } from "./env";
import { apiPeriodParams, type DashboardQuery } from "./query";
import type {
  ApiError,
  Breakdown,
  Interval,
  Realtime,
  Site,
  SiteWithStats,
  Summary,
  Timeseries,
  Timezones,
  Usage,
} from "./types";

const TIMEOUT_MS = 15_000;

/** A non-2xx answer from the Go API. */
export class ApiRequestError extends Error {
  constructor(
    readonly status: number,
    readonly body: ApiError,
  ) {
    super(`tally API ${status}: ${body.error}`);
  }
}

type CallInit = { method?: string; body?: unknown; headers?: Record<string, string> };

async function call<T>(path: string, init: CallInit = {}): Promise<T> {
  const env = loadEnv();
  const res = await fetch(env.apiUrl + path, {
    method: init.method ?? "GET",
    cache: "no-store",
    redirect: "error",
    headers: {
      Authorization: `Bearer ${env.apiSecret}`,
      Accept: "application/json",
      ...(init.body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
    body: init.body !== undefined ? JSON.stringify(init.body) : null,
    signal: AbortSignal.timeout(TIMEOUT_MS),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let json: unknown = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = null;
  }
  if (!res.ok) {
    const body = (json as ApiError | null) ?? { error: `http_${res.status}` };
    throw new ApiRequestError(res.status, body);
  }
  return json as T;
}

const site = (id: string) => `/v1/sites/${encodeURIComponent(id)}`;

export type SiteInput = { name: string; domain: string; timezone: string };

export const api = {
  sites: () => call<{ sites: SiteWithStats[] }>("/v1/sites").then((r) => r.sites),
  site: (id: string) => call<Site>(site(id)),
  createSite: (input: SiteInput) => call<Site>("/v1/sites", { method: "POST", body: input }),
  updateSite: (id: string, patch: Partial<SiteInput>) =>
    call<Site>(site(id), { method: "PATCH", body: patch }),
  deleteSite: (id: string) => call<void>(site(id), { method: "DELETE" }),

  summary: (id: string, q: DashboardQuery) =>
    call<Summary>(`${site(id)}/summary?${apiPeriodParams(q)}`),
  timeseries: (id: string, q: DashboardQuery, interval?: Interval) => {
    const p = apiPeriodParams(q);
    if (interval) p.set("interval", interval);
    return call<Timeseries>(`${site(id)}/timeseries?${p}`);
  },
  breakdown: (id: string, q: DashboardQuery, dimension: string, limit = 10, search = "") => {
    const p = apiPeriodParams(q);
    p.set("dimension", dimension);
    p.set("limit", String(limit));
    if (search) p.set("search", search);
    return call<Breakdown>(`${site(id)}/breakdown?${p}`);
  },
  realtime: (id: string) => call<Realtime>(`${site(id)}/realtime`),
  usage: () => call<Usage>("/v1/usage"),
  timezones: () => call<Timezones>("/v1/timezones"),

  /** Checks the admin password. visitorIp is passed on so the API limits attempts per visitor. */
  login: (password: string, visitorIp: string | null) =>
    call<void>("/v1/login", {
      method: "POST",
      body: { password },
      headers: visitorIp ? { "X-Tally-Visitor-IP": visitorIp } : {},
    }),
};
