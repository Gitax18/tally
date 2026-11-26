// Server configuration. These values are secrets (except API_URL) and must never reach the
// browser: only server code imports this module.

export type Env = {
  /** The Go service, e.g. https://a.example.com. Also the origin of the tracker snippet. */
  apiUrl: string;
  /** Bearer token for the Go API. */
  apiSecret: string;
  /** Signs the dashboard's session cookie. */
  sessionSecret: string;
};

const MIN_SECRET = 32;

/** Reads and checks the environment. Throws a message naming every problem, never a value. */
export function loadEnv(source: Record<string, string | undefined> = process.env): Env {
  const problems: string[] = [];
  const apiUrl = (source.API_URL ?? "").trim().replace(/\/+$/, "");
  const apiSecret = (source.API_SECRET ?? "").trim();
  const sessionSecret = (source.SESSION_SECRET ?? "").trim();

  try {
    const u = new URL(apiUrl);
    if (u.protocol !== "https:" && u.protocol !== "http:") throw new Error();
  } catch {
    problems.push("API_URL must be the Go service's URL, e.g. https://a.example.com");
  }
  if (apiSecret.length < MIN_SECRET)
    problems.push(`API_SECRET must be at least ${MIN_SECRET} characters`);
  if (sessionSecret.length < MIN_SECRET) {
    problems.push(
      `SESSION_SECRET must be at least ${MIN_SECRET} characters (openssl rand -hex 32)`,
    );
  }
  if (problems.length > 0) throw new Error(`Configuration error: ${problems.join("; ")}`);
  return { apiUrl, apiSecret, sessionSecret };
}
