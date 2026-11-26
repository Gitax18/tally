// Captures the README screenshots from a running local stack with seeded data:
//
//   make db migrate seed run      # Go service on :8090 with the Demo site
//   make web-dev                  # or `pnpm build && pnpm start` with the same env
//   node scripts/screenshots.mjs  # writes ../docs/screenshots/*.png
//
// DASHBOARD_URL and ADMIN_PASSWORD default to the Makefile's development values.
import { mkdir } from "node:fs/promises";
import { chromium } from "@playwright/test";

const base = process.env.DASHBOARD_URL ?? "http://127.0.0.1:3010";
const password = process.env.ADMIN_PASSWORD ?? "tally-dev-password";
const out = new URL("../../docs/screenshots/", import.meta.url).pathname;
await mkdir(out, { recursive: true });

const browser = await chromium.launch();
for (const scheme of ["light", "dark"]) {
  const page = await browser.newPage({
    viewport: { width: 1280, height: 900 },
    colorScheme: scheme,
    deviceScaleFactor: 2,
  });
  await page.goto(`${base}/login`);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("heading", { name: "Sites" }).waitFor();

  const demo = page.getByRole("link", { name: /demo\.example/ });
  const href = await demo.getAttribute("href");
  if (scheme === "light") await page.screenshot({ path: `${out}/sites.png` });

  await page.goto(`${base}${href}?period=30d`);
  await page.getByTestId("value-visitors").waitFor();
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${out}/dashboard-${scheme}.png`, fullPage: true });

  if (scheme === "light") {
    await page.goto(`${base}${href}?period=30d&f=source:Hacker News&metric=pageviews`);
    await page.getByTestId("value-visitors").waitFor();
    await page.waitForTimeout(500);
    await page.screenshot({ path: `${out}/dashboard-filtered.png` });
    await page.goto(`${base}${href}?period=30d`);
    await page.getByTestId("panel-pages").getByRole("button", { name: "Details" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "/docs", exact: true }).waitFor();
    await page.screenshot({ path: `${out}/details.png` });
  }
  await page.close();
}
await browser.close();
console.log(`screenshots in ${out}`);
