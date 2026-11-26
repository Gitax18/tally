import { describe, expect, it } from "vitest";
import { loadEnv } from "./env";
import { bucketLabel, formatChange, formatDuration, formatMetric, formatNumber } from "./format";
import {
  apiFilters,
  apiPeriodParams,
  parseQuery,
  toSearch,
  withFilter,
  withoutFilter,
} from "./query";
import { createSessionToken, verifySessionToken } from "./session";

describe("dashboard URL state", () => {
  it("parses and round-trips", () => {
    const q = parseQuery(
      new URLSearchParams("period=7d&f=source:GitHub&f=page:/blog,a:b&metric=pageviews"),
    );
    expect(q).toEqual({
      period: "7d",
      filters: [
        { dim: "source", value: "GitHub" },
        { dim: "page", value: "/blog,a:b" },
      ],
      metric: "pageviews",
    });
    expect(parseQuery(new URLSearchParams(toSearch(q).slice(1)))).toEqual(q);
  });

  it("falls back to defaults on anything malformed", () => {
    const q = parseQuery({
      period: "week",
      f: ["nope:1", "source:A", "source:B", "country"],
      metric: "x",
    });
    expect(q).toEqual({
      period: "30d",
      filters: [{ dim: "source", value: "A" }],
      metric: "visitors",
    });
    expect(parseQuery({ period: "custom", from: "2026-11-01" }).period).toBe("30d");
    expect(parseQuery({ period: "custom", from: "2026-11-01", to: "2026-11-05" })).toMatchObject({
      period: "custom",
      from: "2026-11-01",
      to: "2026-11-05",
    });
    expect(toSearch(parseQuery({}))).toBe("");
  });

  it("encodes values for the Go API so commas and colons survive", () => {
    expect(
      apiFilters([
        { dim: "page", value: "/a,b:c" },
        { dim: "country", value: "IN" },
      ]),
    ).toBe("page:%2Fa%2Cb%3Ac,country:IN");
    const p = apiPeriodParams(
      parseQuery({ period: "custom", from: "2026-11-01", to: "2026-11-05", f: "source:X" }),
    );
    expect(p.toString()).toBe("period=custom&from=2026-11-01&to=2026-11-05&filters=source%3AX");
  });

  it("replaces a filter on the same dimension and removes one", () => {
    let q = parseQuery({ f: ["source:A", "country:IN"] });
    q = withFilter(q, { dim: "source", value: "B" });
    expect(q.filters).toEqual([
      { dim: "country", value: "IN" },
      { dim: "source", value: "B" },
    ]);
    expect(withoutFilter(q, "country").filters).toEqual([{ dim: "source", value: "B" }]);
  });
});

describe("formatting", () => {
  it("formats numbers, durations and metrics", () => {
    expect(formatNumber(950)).toBe("950");
    expect(formatNumber(1234)).toBe("1.2K");
    expect(formatDuration(42)).toBe("42s");
    expect(formatDuration(185)).toBe("3m 05s");
    expect(formatDuration(3720)).toBe("1h 02m");
    expect(formatMetric("bounce_rate", 33.3)).toBe("33%");
    expect(formatMetric("views_per_visit", 2.5)).toBe("2.5");
    expect(formatMetric("views_per_visit", 2)).toBe("2");
  });

  it("colours changes the right way round", () => {
    expect(formatChange("visitors", 12.4)).toEqual({ text: "↑12%", tone: "good" });
    expect(formatChange("visitors", -5)).toEqual({ text: "↓5%", tone: "bad" });
    expect(formatChange("bounce_rate", -6.7)).toEqual({ text: "↓7 pp", tone: "good" });
    expect(formatChange("bounce_rate", 3)).toEqual({ text: "↑3 pp", tone: "bad" });
    expect(formatChange("visitors", null)).toBeNull();
  });

  it("labels chart buckets", () => {
    expect(bucketLabel("2026-11-27T00:00", "hour")).toBe("12am");
    expect(bucketLabel("2026-11-27T14:00", "hour", true)).toBe("27 Nov, 2pm");
    expect(bucketLabel("2026-11-27", "day")).toBe("27 Nov");
    expect(bucketLabel("2026-11", "month", true)).toBe("Nov 2026");
  });
});

describe("session", () => {
  const secret = "s".repeat(40);

  it("accepts its own token and rejects others", async () => {
    const token = await createSessionToken(secret);
    expect(await verifySessionToken(token, secret)).toBe(true);
    expect(await verifySessionToken(token, "t".repeat(40))).toBe(false);
    expect(await verifySessionToken(token.slice(0, -2) + "xx", secret)).toBe(false);
    expect(await verifySessionToken(undefined, secret)).toBe(false);
  });

  it("expires after 30 days", async () => {
    const issued = new Date("2026-01-01T00:00:00Z");
    const token = await createSessionToken(secret, issued);
    expect(await verifySessionToken(token, secret, new Date("2026-01-30T00:00:00Z"))).toBe(true);
    expect(await verifySessionToken(token, secret, new Date("2026-02-01T00:00:00Z"))).toBe(false);
  });
});

describe("env", () => {
  it("names every problem without echoing values", () => {
    expect(() =>
      loadEnv({ API_URL: "nope", API_SECRET: "short-secret-value", SESSION_SECRET: "" }),
    ).toThrow(/API_URL.*API_SECRET.*SESSION_SECRET/);
    try {
      loadEnv({ API_URL: "nope", API_SECRET: "short-secret-value" });
    } catch (e) {
      expect(String(e)).not.toContain("short-secret-value");
    }
    expect(
      loadEnv({
        API_URL: "https://a.example.com/",
        API_SECRET: "a".repeat(32),
        SESSION_SECRET: "b".repeat(32),
      }).apiUrl,
    ).toBe("https://a.example.com");
  });
});
