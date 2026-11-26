"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { requireSession, visitorIp } from "@/lib/auth";
import { loadEnv } from "@/lib/env";
import { createSessionToken, SESSION_COOKIE, sessionCookieOptions } from "@/lib/session";
import type { Site } from "@/lib/types";

export type FormState = {
  error?: string;
  fieldErrors?: Record<string, string>;
  site?: Site;
  saved?: boolean;
};

/** Only same-site paths are followed after login, never another origin. */
function safeNext(next: unknown): string {
  return typeof next === "string" && next.startsWith("/") && !next.startsWith("//") ? next : "/";
}

export async function login(_: FormState, form: FormData): Promise<FormState> {
  const password = String(form.get("password") ?? "");
  if (!password) return { error: "Enter the admin password." };
  try {
    await api.login(password, await visitorIp());
  } catch (err) {
    if (err instanceof ApiRequestError && (err.status === 401 || err.status === 429)) {
      return { error: err.body.message ?? "That did not work." };
    }
    return { error: "The tally server could not be reached. Try again in a moment." };
  }
  const token = await createSessionToken(loadEnv().sessionSecret);
  (await cookies()).set(
    SESSION_COOKIE,
    token,
    sessionCookieOptions(process.env.NODE_ENV === "production"),
  );
  redirect(safeNext(form.get("next")));
}

export async function logout(): Promise<void> {
  (await cookies()).delete(SESSION_COOKIE);
  redirect("/login");
}

function siteError(err: unknown): FormState {
  if (err instanceof ApiRequestError && err.status === 422 && err.body.field) {
    return { fieldErrors: { [err.body.field]: err.body.message ?? "This value is not valid." } };
  }
  if (err instanceof ApiRequestError && err.status === 404)
    return { error: "This site no longer exists." };
  return { error: "The tally server could not be reached. Try again in a moment." };
}

function siteInput(form: FormData) {
  return {
    name: String(form.get("name") ?? ""),
    domain: String(form.get("domain") ?? ""),
    timezone: String(form.get("timezone") ?? "UTC"),
  };
}

export async function createSite(_: FormState, form: FormData): Promise<FormState> {
  await requireSession();
  try {
    return { site: await api.createSite(siteInput(form)) };
  } catch (err) {
    return siteError(err);
  }
}

export async function updateSite(_: FormState, form: FormData): Promise<FormState> {
  await requireSession();
  const id = String(form.get("id") ?? "");
  try {
    return { site: await api.updateSite(id, siteInput(form)), saved: true };
  } catch (err) {
    return siteError(err);
  }
}

export async function deleteSite(_: FormState, form: FormData): Promise<FormState> {
  await requireSession();
  const id = String(form.get("id") ?? "");
  const confirm = String(form.get("confirm") ?? "");
  let site: Site;
  try {
    site = await api.site(id);
  } catch (err) {
    return siteError(err);
  }
  if (confirm !== site.name) return { fieldErrors: { confirm: `Type ${site.name} to confirm.` } };
  try {
    await api.deleteSite(id);
  } catch (err) {
    return siteError(err);
  }
  redirect("/");
}
