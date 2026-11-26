import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { DeleteSiteForm, SettingsForm, WaitingForFirstEvent } from "@/components/site-form";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Snippet } from "@/components/ui/copy-button";
import { api, ApiRequestError } from "@/lib/api";
import { loadEnv } from "@/lib/env";
import { snippetFor } from "@/lib/snippet";
import { timezones } from "@/lib/timezones";
import type { Site } from "@/lib/types";

export const metadata: Metadata = { title: "Site settings" };

export default async function SettingsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  let site: Site;
  try {
    site = await api.site(id);
  } catch (err) {
    if (err instanceof ApiRequestError && err.status === 404) notFound();
    throw err;
  }
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-4">
      <Link
        href={`/sites/${site.id}`}
        className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-sm"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {site.name}
      </Link>

      <Card>
        <CardHeader>
          <CardTitle>Tracking snippet</CardTitle>
          <CardDescription>Paste it into the &lt;head&gt; of every page.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Snippet code={snippetFor(loadEnv().apiUrl, site.id)} />
          {site.last_event_at ? null : <WaitingForFirstEvent siteId={site.id} />}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Settings</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingsForm site={site} timezones={(await timezones()).timezones} />
        </CardContent>
      </Card>

      <Card className="border-bad/40">
        <CardHeader>
          <CardTitle>Delete site</CardTitle>
        </CardHeader>
        <CardContent>
          <DeleteSiteForm site={site} />
        </CardContent>
      </Card>
    </div>
  );
}
