"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import * as React from "react";
import { parseQuery, toSearch, type DashboardQuery } from "@/lib/query";
import type { MetricKey } from "@/lib/types";

/**
 * The dashboard state from the URL, and ways to change it. Period and filter changes navigate
 * (the server fetches new numbers); switching the chart's metric only rewrites the URL, because
 * the chart already has every metric.
 */
export function useDashboard() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const query = React.useMemo(() => parseQuery(new URLSearchParams(params.toString())), [params]);
  const [pending, startTransition] = React.useTransition();

  const navigate = React.useCallback(
    (next: DashboardQuery) => {
      startTransition(() => router.push(pathname + toSearch(next), { scroll: false }));
    },
    [router, pathname],
  );

  const setMetric = React.useCallback(
    (metric: MetricKey) => {
      window.history.replaceState(null, "", pathname + toSearch({ ...query, metric }));
    },
    [pathname, query],
  );

  return { query, navigate, setMetric, pending };
}
