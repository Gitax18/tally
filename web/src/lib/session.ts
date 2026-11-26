// The dashboard's session: a JWT signed with SESSION_SECRET, kept in an httpOnly cookie for 30
// days. There is one user, so the token says nothing but "signed in" and when it expires.
//
// This module is imported by proxy.ts as well as by server code, so it does not import
// "server-only" (which the proxy bundle cannot load). It never runs in the browser.
import { jwtVerify, SignJWT } from "jose";

export const SESSION_COOKIE = "tally_session";
export const SESSION_DAYS = 30;

const ISSUER = "tally-dashboard";

function key(secret: string): Uint8Array {
  return new TextEncoder().encode(secret);
}

export async function createSessionToken(secret: string, now = new Date()): Promise<string> {
  const iat = Math.floor(now.getTime() / 1000);
  return new SignJWT({})
    .setProtectedHeader({ alg: "HS256" })
    .setIssuer(ISSUER)
    .setSubject("admin")
    .setIssuedAt(iat)
    .setExpirationTime(iat + SESSION_DAYS * 24 * 60 * 60)
    .sign(key(secret));
}

/** True when token is a valid, unexpired session signed with secret. */
export async function verifySessionToken(
  token: string | undefined,
  secret: string,
  now = new Date(),
): Promise<boolean> {
  if (!token) return false;
  try {
    await jwtVerify(token, key(secret), {
      issuer: ISSUER,
      subject: "admin",
      algorithms: ["HS256"],
      currentDate: now,
    });
    return true;
  } catch {
    return false;
  }
}

export function sessionCookieOptions(secure: boolean) {
  return {
    httpOnly: true,
    secure,
    sameSite: "lax" as const,
    path: "/",
    maxAge: SESSION_DAYS * 24 * 60 * 60,
  };
}
