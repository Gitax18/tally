import { AlertTriangle, Plus } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { Sparkline } from "@/components/sparkline";
import { buttonVariants } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { api } from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/format";
import type { Usage } from "@/lib/types";
import { cn } from "@/lib/utils";

export const metadata: Metadata = { title: "Sites" };

export default async function SitesPage() {
  const [sites, usage] = await Promise.all([api.sites(), api.usage()]);
  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-xl font-semibold tracking-tight">Sites</h1>
        <Link href="/sites/new" className={buttonVariants({ variant: "accent", size: "sm" })}>
          <Plus aria-hidden />
          Add site
        </Link>
      </div>

      {sites.length === 0 ? (
        <Card className="flex flex-col items-center gap-3 px-6 py-14 text-center">
          <p className="font-medium">No sites yet</p>
          <p className="text-muted-foreground max-w-sm text-sm">
            Add a site to get its tracking snippet. Visits show up here as soon as the snippet is on
            a page.
          </p>
          <Link href="/sites/new" className={buttonVariants({ variant: "accent", size: "sm" })}>
            Add your first site
          </Link>
        </Card>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {sites.map((s) => (
            <li key={s.id}>
              <Link
                href={`/sites/${s.id}`}
                className="group focus-visible:outline-accent block rounded-card focus-visible:outline-2 focus-visible:outline-offset-2"
              >
                <Card className="group-hover:border-border-strong flex flex-col gap-4 p-5 transition-colors">
                  <div className="min-w-0">
                    <p className="truncate font-semibold">{s.name}</p>
                    <p className="text-muted-foreground truncate text-sm">{s.domain}</p>
                  </div>
                  <div className="flex items-end justify-between gap-4">
                    <div>
                      <p className="tabular text-2xl font-semibold">
                        {formatNumber(s.visitors_24h)}
                      </p>
                      <p className="text-muted-foreground text-xs">visitors in the last 24 hours</p>
                    </div>
                    <Sparkline
                      values={s.sparkline}
                      label={`Daily visitors over 7 days: ${s.sparkline.join(", ")}`}
                    />
                  </div>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <StorageMeter usage={usage} />
    </div>
  );
}

function StorageMeter({ usage }: { usage: Usage }) {
  const pct = Math.min(100, usage.percent);
  return (
    <Card className="flex flex-col gap-3 p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
        <span className="font-medium">Database storage</span>
        <span className="text-muted-foreground tabular">
          {formatBytes(usage.database_bytes)} of {formatBytes(usage.limit_bytes)} ({usage.percent}%)
        </span>
      </div>
      <div
        className="bg-surface-2 h-2 overflow-hidden rounded-full"
        role="meter"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        aria-label="Database storage used"
      >
        <div
          className={cn("h-full rounded-full", usage.warn ? "bg-warn" : "bg-accent")}
          style={{ width: `${pct}%` }}
        />
      </div>
      {usage.warn ? (
        <p className="text-warn flex items-start gap-2 text-sm" role="alert">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
          The database is over 80% full. Heroku Essential plans stop accepting writes when full, so
          new events would be lost: upgrade the plan or lower RETENTION_MONTHS.
        </p>
      ) : null}
    </Card>
  );
}
