import "server-only";
import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { loadEnv } from "./env";
import { SESSION_COOKIE, verifySessionToken } from "./session";

/** True when the request carries a valid session. */
export async function isSignedIn(): Promise<boolean> {
  const token = (await cookies()).get(SESSION_COOKIE)?.value;
  return verifySessionToken(token, loadEnv().sessionSecret);
}

/**
 * Server actions and route handlers check the session themselves too: the proxy is the first
 * line, this is the second.
 */
export async function requireSession(): Promise<void> {
  if (!(await isSignedIn())) redirect("/login");
}

/**
 * The visitor's address, passed to the Go API so login attempts are limited per visitor. Only
 * believed where the host sets X-Forwarded-For itself (Vercel does, overwriting what the
 * client sent); elsewhere it is unknown and the API limits by this server's address.
 */
export async function visitorIp(): Promise<string | null> {
  if (process.env.VERCEL !== "1" && process.env.TRUST_FORWARDED_FOR !== "true") return null;
  const h = await headers();
  const ip = h.get("x-real-ip") ?? h.get("x-forwarded-for")?.split(",")[0];
  return ip?.trim() || null;
}
