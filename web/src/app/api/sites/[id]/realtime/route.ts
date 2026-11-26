import { NextResponse } from "next/server";
import { api, ApiRequestError } from "@/lib/api";
import { isSignedIn } from "@/lib/auth";

// Polled by the realtime counter and the "waiting for the first event" check.
export async function GET(_: Request, ctx: { params: Promise<{ id: string }> }) {
  if (!(await isSignedIn())) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const { id } = await ctx.params;
  try {
    return NextResponse.json(await api.realtime(id), { headers: { "Cache-Control": "no-store" } });
  } catch (err) {
    const status = err instanceof ApiRequestError ? err.status : 502;
    return NextResponse.json({ error: "unavailable" }, { status });
  }
}
