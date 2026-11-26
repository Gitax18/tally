import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { NewSiteForm } from "@/components/site-form";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { loadEnv } from "@/lib/env";
import { timezones } from "@/lib/timezones";

export const metadata: Metadata = { title: "Add site" };

export default async function NewSitePage() {
  const zones = await timezones();
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-4">
      <Link
        href="/"
        className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-sm"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Sites
      </Link>
      <Card>
        <CardHeader>
          <CardTitle>Add a site</CardTitle>
          <CardDescription>You get a one-line snippet to paste into the site.</CardDescription>
        </CardHeader>
        <CardContent>
          <NewSiteForm apiUrl={loadEnv().apiUrl} zones={zones} />
        </CardContent>
      </Card>
    </div>
  );
}
