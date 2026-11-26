import { countryFlag, countryName } from "./format";

export type Column = "visitors" | "pageviews" | "visits" | "time_on_page_ms";

export type DimensionInfo = {
  label: string;
  /** Shown for an empty value. */
  empty: string;
  /** Columns after the value, the first is the bar. */
  columns: Column[];
  /** Header names for the columns. */
  headers: string[];
};

export const DIMENSIONS: Record<string, DimensionInfo> = {
  source: { label: "Source", empty: "Direct / None", columns: ["visitors"], headers: ["Visitors"] },
  referrer: { label: "Referrer", empty: "(none)", columns: ["visitors"], headers: ["Visitors"] },
  utm_source: {
    label: "UTM source",
    empty: "(none)",
    columns: ["visitors"],
    headers: ["Visitors"],
  },
  utm_medium: {
    label: "UTM medium",
    empty: "(none)",
    columns: ["visitors"],
    headers: ["Visitors"],
  },
  utm_campaign: {
    label: "UTM campaign",
    empty: "(none)",
    columns: ["visitors"],
    headers: ["Visitors"],
  },
  page: {
    label: "Page",
    empty: "/",
    columns: ["visitors", "pageviews", "time_on_page_ms"],
    headers: ["Visitors", "Views", "Time"],
  },
  entry_page: {
    label: "Entry page",
    empty: "/",
    columns: ["visitors", "visits"],
    headers: ["Visitors", "Entrances"],
  },
  exit_page: {
    label: "Exit page",
    empty: "/",
    columns: ["visitors", "visits"],
    headers: ["Visitors", "Exits"],
  },
  country: { label: "Country", empty: "Unknown", columns: ["visitors"], headers: ["Visitors"] },
  browser: { label: "Browser", empty: "Unknown", columns: ["visitors"], headers: ["Visitors"] },
  os: { label: "OS", empty: "Unknown", columns: ["visitors"], headers: ["Visitors"] },
  device: { label: "Device", empty: "Unknown", columns: ["visitors"], headers: ["Visitors"] },
  hostname: { label: "Hostname", empty: "Unknown", columns: ["visitors"], headers: ["Visitors"] },
};

/** How a value reads on screen. */
export function displayValue(dim: string, value: string): string {
  const info = DIMENSIONS[dim];
  if (value === "") return info?.empty ?? "(none)";
  if (dim === "country") return `${countryFlag(value)} ${countryName(value)}`.trim();
  return value;
}

export type Tab = { dim: string; label: string };

export type Panel = { id: string; title: string; tabs: Tab[] };

/** The four panels under the chart. */
export const PANELS: Panel[] = [
  {
    id: "sources",
    title: "Sources",
    tabs: [
      { dim: "source", label: "Source" },
      { dim: "referrer", label: "Referrer" },
      { dim: "utm_source", label: "UTM source" },
      { dim: "utm_medium", label: "Medium" },
      { dim: "utm_campaign", label: "Campaign" },
    ],
  },
  {
    id: "pages",
    title: "Pages",
    tabs: [
      { dim: "page", label: "Top pages" },
      { dim: "entry_page", label: "Entry" },
      { dim: "exit_page", label: "Exit" },
    ],
  },
  { id: "countries", title: "Countries", tabs: [{ dim: "country", label: "Country" }] },
  {
    id: "devices",
    title: "Devices",
    tabs: [
      { dim: "browser", label: "Browser" },
      { dim: "os", label: "OS" },
      { dim: "device", label: "Device" },
    ],
  },
];
