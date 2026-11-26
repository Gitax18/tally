"use client";

import { CheckCircle2, Loader2 } from "lucide-react";
import Link from "next/link";
import * as React from "react";
import { createSite, deleteSite, updateSite, type FormState } from "@/app/actions";
import { Button, buttonVariants } from "@/components/ui/button";
import { Snippet } from "@/components/ui/copy-button";
import { Field, Input, Select } from "@/components/ui/input";
import { snippetFor } from "@/lib/snippet";
import type { Realtime, Site, Timezones } from "@/lib/types";

const noop = () => () => {};

/**
 * The browser's time zone after hydration (UTC on the server), under its current name: browsers
 * still report some zones by legacy names, such as Asia/Calcutta for Asia/Kolkata.
 */
function useBrowserTimezone({ timezones, aliases }: Timezones): string {
  const reported = React.useSyncExternalStore(
    noop,
    () => Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    () => "UTC",
  );
  const current = aliases[reported] ?? reported;
  return timezones.includes(current) ? current : "UTC";
}

function SiteFields({
  state,
  site,
  timezones,
  defaultTimezone,
}: {
  state: FormState;
  site?: Site;
  timezones: string[];
  defaultTimezone: string;
}) {
  const err = state.fieldErrors ?? {};
  // A site saved before the list changed still shows its own zone.
  const options = timezones.includes(defaultTimezone) ? timezones : [defaultTimezone, ...timezones];
  return (
    <>
      <Field label="Name" htmlFor="name" error={err.name}>
        <Input
          id="name"
          name="name"
          required
          maxLength={100}
          defaultValue={site?.name}
          placeholder="My blog"
        />
      </Field>
      <Field
        label="Domain"
        htmlFor="domain"
        error={err.domain}
        hint="Subdomains are included. Use localhost for a test site."
      >
        <Input
          id="domain"
          name="domain"
          required
          defaultValue={site?.domain}
          placeholder="example.com"
          autoCapitalize="none"
          spellCheck={false}
        />
      </Field>
      <Field
        label="Time zone"
        htmlFor="timezone"
        error={err.timezone}
        hint="Days and hours on the charts follow this zone."
      >
        <Select
          id="timezone"
          name="timezone"
          key={defaultTimezone}
          defaultValue={site?.timezone ?? defaultTimezone}
        >
          {options.map((tz) => (
            <option key={tz} value={tz}>
              {tz}
            </option>
          ))}
        </Select>
      </Field>
    </>
  );
}

export function NewSiteForm({ apiUrl, zones }: { apiUrl: string; zones: Timezones }) {
  const [state, action, pending] = React.useActionState<FormState, FormData>(createSite, {});
  const tz = useBrowserTimezone(zones);
  if (state.site) {
    return <SiteCreated site={state.site} apiUrl={apiUrl} />;
  }
  return (
    <form action={action} className="flex flex-col gap-5">
      <SiteFields state={state} timezones={zones.timezones} defaultTimezone={tz} />
      {state.error ? (
        <p className="text-bad text-sm" role="alert">
          {state.error}
        </p>
      ) : null}
      <div>
        <Button type="submit" variant="accent" busy={pending}>
          Add site
        </Button>
      </div>
    </form>
  );
}

function SiteCreated({ site, apiUrl }: { site: Site; apiUrl: string }) {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        <h2 className="font-semibold">
          Paste this into the &lt;head&gt; of every page on {site.domain}
        </h2>
        <p className="text-muted-foreground text-sm">
          One line, no cookies, no banner needed. It loads in the background and is under 1&nbsp;KB.
        </p>
      </div>
      <Snippet code={snippetFor(apiUrl, site.id)} />
      <WaitingForFirstEvent siteId={site.id} />
    </div>
  );
}

/** Polls until the site's first event arrives. */
export function WaitingForFirstEvent({
  siteId,
  initial = null,
}: {
  siteId: string;
  initial?: string | null;
}) {
  const [lastEvent, setLastEvent] = React.useState<string | null>(initial);
  React.useEffect(() => {
    if (lastEvent) return;
    let stopped = false;
    const tick = async () => {
      try {
        const res = await fetch(`/api/sites/${siteId}/realtime`, { cache: "no-store" });
        if (res.ok) {
          const data = (await res.json()) as Realtime;
          if (!stopped && data.last_event_at) setLastEvent(data.last_event_at);
        }
      } catch {
        // Try again on the next tick.
      }
    };
    const timer = setInterval(tick, 3000);
    void tick();
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [siteId, lastEvent]);

  if (lastEvent) {
    return (
      <div
        className="border-ok/40 bg-ok/5 rounded-control flex flex-wrap items-center justify-between gap-3 border p-4"
        role="status"
      >
        <p className="text-ok flex items-center gap-2 text-sm font-medium">
          <CheckCircle2 className="size-4" aria-hidden />
          Receiving events.
        </p>
        <Link
          href={`/sites/${siteId}`}
          className={buttonVariants({ variant: "accent", size: "sm" })}
        >
          Open the dashboard
        </Link>
      </div>
    );
  }
  return (
    <div
      className="border-border rounded-control flex flex-wrap items-center justify-between gap-3 border border-dashed p-4"
      role="status"
    >
      <p className="text-muted-foreground flex items-center gap-2 text-sm">
        <Loader2 className="size-4 animate-spin" aria-hidden />
        Waiting for the first event… visit a page with the snippet.
      </p>
      <Link href={`/sites/${siteId}`} className={buttonVariants({ variant: "ghost", size: "sm" })}>
        Skip
      </Link>
    </div>
  );
}

export function SettingsForm({ site, timezones }: { site: Site; timezones: string[] }) {
  const [state, action, pending] = React.useActionState<FormState, FormData>(updateSite, {});
  const current = state.site ?? site;
  return (
    <form action={action} className="flex flex-col gap-5">
      <input type="hidden" name="id" value={site.id} />
      <SiteFields
        state={state}
        site={current}
        timezones={timezones}
        defaultTimezone={current.timezone}
      />
      {state.error ? (
        <p className="text-bad text-sm" role="alert">
          {state.error}
        </p>
      ) : null}
      <div className="flex items-center gap-3">
        <Button type="submit" busy={pending}>
          Save changes
        </Button>
        {state.saved && !pending ? (
          <span className="text-ok text-sm" role="status">
            Saved.
          </span>
        ) : null}
      </div>
    </form>
  );
}

export function DeleteSiteForm({ site }: { site: Site }) {
  const [state, action, pending] = React.useActionState<FormState, FormData>(deleteSite, {});
  const [typed, setTyped] = React.useState("");
  return (
    <form action={action} className="flex flex-col gap-4">
      <input type="hidden" name="id" value={site.id} />
      <Field
        label={`Type ${site.name} to confirm`}
        htmlFor="confirm"
        error={state.fieldErrors?.confirm ?? state.error}
        hint="This deletes the site and all of its data. It cannot be undone."
      >
        <Input
          id="confirm"
          name="confirm"
          autoComplete="off"
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
        />
      </Field>
      <div>
        <Button type="submit" variant="danger" busy={pending} disabled={typed !== site.name}>
          Delete site
        </Button>
      </div>
    </form>
  );
}
