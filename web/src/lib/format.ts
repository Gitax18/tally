import type { MetricKey } from "./types";

const compact = new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 1 });
const whole = new Intl.NumberFormat("en");

/** 950, 1.2K, 3.4M. */
export function formatNumber(n: number): string {
  return Math.abs(n) < 1000 ? whole.format(n) : compact.format(n);
}

/** 42s, 3m 05s, 1h 02m. */
export function formatDuration(seconds: number): string {
  const s = Math.round(seconds);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(s / 3600)}h ${String(Math.floor((s % 3600) / 60)).padStart(2, "0")}m`;
}

export function formatMetric(key: MetricKey, value: number): string {
  switch (key) {
    case "bounce_rate":
      return `${Math.round(value)}%`;
    case "visit_duration":
      return formatDuration(value);
    case "views_per_visit":
      return value.toFixed(2).replace(/\.?0+$/, "") || "0";
    default:
      return formatNumber(value);
  }
}

export type ChangeView = { text: string; tone: "good" | "bad" | "neutral" };

/**
 * The change against the previous period. Bounce rate changes are percentage points and a fall
 * is good; for everything else a rise is good.
 */
export function formatChange(key: MetricKey, change: number | null): ChangeView | null {
  if (change === null) return null;
  const rounded = Math.round(change);
  const unit = key === "bounce_rate" ? " pp" : "%";
  const arrow = rounded > 0 ? "↑" : rounded < 0 ? "↓" : "";
  const text = `${arrow}${Math.abs(rounded)}${unit}`;
  if (rounded === 0) return { text: `0${unit}`, tone: "neutral" };
  const up = rounded > 0;
  const good = key === "bounce_rate" ? !up : up;
  return { text, tone: good ? "good" : "bad" };
}

export function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}

const regionNames = new Intl.DisplayNames(["en"], { type: "region" });

/** "IN" → "India"; unknown codes stay as they are. */
export function countryName(code: string): string {
  if (!/^[A-Z]{2}$/.test(code)) return code;
  try {
    return regionNames.of(code) ?? code;
  } catch {
    return code;
  }
}

/** "IN" → 🇮🇳 */
export function countryFlag(code: string): string {
  if (!/^[A-Z]{2}$/.test(code)) return "";
  return String.fromCodePoint(...[...code].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65));
}

/** A chart axis label for a bucket key from the API ("2026-11-27T14:00", "2026-11-27", "2026-11"). */
export function bucketLabel(
  bucket: string,
  interval: "hour" | "day" | "month",
  long = false,
): string {
  const months = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
  ];
  const [date = "", time = ""] = bucket.split("T");
  const [y, m, d] = date.split("-").map(Number);
  const month = months[(m ?? 1) - 1] ?? "";
  if (interval === "month") return long ? `${month} ${y}` : month;
  if (interval === "day") return `${d} ${month}`;
  const hour = Number(time.slice(0, 2));
  const label = `${hour === 0 ? 12 : hour > 12 ? hour - 12 : hour}${hour < 12 ? "am" : "pm"}`;
  return long ? `${d} ${month}, ${label}` : label;
}
