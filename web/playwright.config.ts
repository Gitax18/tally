import { defineConfig } from "@playwright/test";

// The end-to-end test runs the whole stack: the Go service (on the test database), a static
// server for examples/, and the built dashboard. Postgres must be running (`make db` or CI's
// service container). Build the dashboard first: `pnpm build && pnpm test:e2e`.
const DATABASE_URL =
  process.env.E2E_DATABASE_URL ??
  "postgres://tally:tally@127.0.0.1:5442/tally_test?sslmode=disable";
export const API_PORT = 8091;
export const EXAMPLES_PORT = 8098;
export const WEB_PORT = 3011;
export const ADMIN_PASSWORD = "e2e-admin-password";
const API_SECRET = "e2e-api-secret-0123456789abcdef0123456789";

// A desktop Chrome user agent for the whole browser. A context-level override is not enough:
// beacons sent while a page unloads use the browser's own user agent, which says HeadlessChrome
// and is (rightly) dropped as a bot.
const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36";

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: `http://127.0.0.1:${WEB_PORT}`,
    viewport: { width: 1280, height: 900 },
    launchOptions: {
      // Without this navigator.webdriver is true and the tracker ignores the browser on purpose.
      args: ["--disable-blink-features=AutomationControlled", `--user-agent=${UA}`],
    },
  },
  webServer: [
    {
      cwd: "..",
      command:
        "go run ./cmd/migrate up && " +
        'ADMIN_PASSWORD_HASH="$(printf \'%s\\n\' "$E2E_PASSWORD" | go run ./cmd/server hash-password)" go run ./cmd/server',
      env: {
        DATABASE_URL,
        API_SECRET,
        E2E_PASSWORD: ADMIN_PASSWORD,
        PORT: String(API_PORT),
        LISTEN_HOST: "127.0.0.1",
        GEOIP_DOWNLOAD: "false",
      },
      url: `http://127.0.0.1:${API_PORT}/healthz`,
      // The service logs to stdout; show it, so a failing API call explains itself.
      stdout: "pipe",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      command: `node e2e/static-server.mjs ${EXAMPLES_PORT}`,
      url: `http://127.0.0.1:${EXAMPLES_PORT}/demo.html`,
      reuseExistingServer: false,
    },
    {
      command: `pnpm exec next start --hostname 127.0.0.1 --port ${WEB_PORT}`,
      env: {
        API_URL: `http://127.0.0.1:${API_PORT}`,
        API_SECRET,
        SESSION_SECRET: "e2e-session-secret-0123456789abcdef0123",
      },
      url: `http://127.0.0.1:${WEB_PORT}/login`,
      timeout: 60_000,
      reuseExistingServer: false,
    },
  ],
});
