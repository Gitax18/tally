import { NextResponse, type NextRequest } from "next/server";
import { api, ApiRequestError } from "@/lib/api";
import { isSignedIn } from "@/lib/auth";
import { parseQuery } from "@/lib/query";

// Used by the "details" modal: the full list for one dimension, with search.
// Takes the dashboard's own URL parameters (period, from, to, f) plus dimension, search, limit.
export async function GET(request: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  if (!(await isSignedIn())) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const { id } = await ctx.params;
  const sp = request.nextUrl.searchParams;
  const q = parseQuery(sp);
  const limit = Math.min(1000, Math.max(1, Number(sp.get("limit")) || 100));
  try {
    const res = await api.breakdown(
      id,
      q,
      sp.get("dimension") ?? "",
      limit,
      (sp.get("search") ?? "").slice(0, 200),
    );
    return NextResponse.json(res, { headers: { "Cache-Control": "no-store" } });
  } catch (err) {
    if (err instanceof ApiRequestError) return NextResponse.json(err.body, { status: err.status });
    return NextResponse.json({ error: "unavailable" }, { status: 502 });
  }
}
