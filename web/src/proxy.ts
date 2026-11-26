import { NextResponse, type NextRequest } from "next/server";
import { buildCsp, newNonce } from "@/lib/csp";
import { SESSION_COOKIE, verifySessionToken } from "@/lib/session";

/**
 * Runs before every page and route handler:
 *   - anything but /login needs a valid session cookie; pages redirect to /login, and /api/*
 *     route handlers answer 401;
 *   - every response gets a Content-Security-Policy with a fresh nonce.
 */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const { pathname, search } = request.nextUrl;

  if (pathname !== "/login") {
    const secret = process.env.SESSION_SECRET ?? "";
    const token = request.cookies.get(SESSION_COOKIE)?.value;
    const ok = secret.length >= 32 && (await verifySessionToken(token, secret));
    if (!ok) {
      if (pathname.startsWith("/api/")) {
        return NextResponse.json({ error: "unauthorized" }, { status: 401 });
      }
      const url = request.nextUrl.clone();
      url.pathname = "/login";
      url.search = "";
      if (pathname !== "/") url.searchParams.set("next", pathname + search);
      return NextResponse.redirect(url);
    }
  }

  const nonce = newNonce();
  const csp = buildCsp(nonce, process.env.NODE_ENV === "development");
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", csp);
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", csp);
  return response;
}

export const config = {
  matcher: [
    // Static assets carry no data and need no session.
    "/((?!_next/static|_next/image|favicon.ico|icon.svg).*)",
  ],
};
