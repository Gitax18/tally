import { expect, test, type Page } from "@playwright/test";
import { ADMIN_PASSWORD, API_PORT, EXAMPLES_PORT } from "../playwright.config";

const shots = "test-results/screens";

async function signIn(page: Page) {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await page.getByLabel("Password").fill("not the password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByText("That password is not correct.")).toBeVisible();
  await page.getByLabel("Password").fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Sites" })).toBeVisible();
}

test("a visit to a tracked page shows up on the dashboard", async ({ page, context }) => {
  await signIn(page);

  // Add a localhost site through the UI.
  const name = `E2E ${Date.now()}`;
  await page
    .getByRole("link", { name: /add (your first )?site/i })
    .first()
    .click();
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Domain").fill("localhost");
  await page.getByRole("button", { name: "Add site" }).click();
  const snippet = page.locator("code");
  await expect(snippet).toContainText(`http://127.0.0.1:${API_PORT}/s/`);
  const siteId = (await snippet.textContent())!.match(/\/s\/([a-z0-9]{10})\.js/)![1]!;
  await expect(page.getByRole("status")).toContainText("Waiting for the first event");
  await page.screenshot({ path: `${shots}/add-site.png` });

  // A visitor opens the demo with a campaign, navigates the SPA, and leaves.
  const visitor = await context.newPage();
  await visitor.goto(
    `http://localhost:${EXAMPLES_PORT}/demo.html?site=${siteId}&api=http://127.0.0.1:${API_PORT}&utm_source=test&token=secret`,
  );
  await expect(visitor.locator("#status")).toContainText("Tracking");
  await visitor.waitForTimeout(1500);
  await visitor.getByTestId("spa-about").click();
  await visitor.waitForTimeout(1500);
  await visitor.getByTestId("spa-pricing").click();
  await visitor.waitForTimeout(2000);
  await visitor.close({ runBeforeUnload: true });

  // The add-site page notices the first event on its own.
  await expect(page.getByRole("status")).toContainText("Receiving events", { timeout: 15_000 });
  await page.getByRole("link", { name: "Open the dashboard" }).click();
  await expect(page).toHaveURL(new RegExp(`/sites/${siteId}$`));

  // Today's numbers: one visitor, three pageviews, engaged time, source "test", one current visitor.
  await page.goto(`/sites/${siteId}?period=today`);
  await expect
    .poll(async () => {
      await page.reload();
      return page.getByTestId("value-visit_duration").textContent();
    })
    .not.toBe("0s");
  await expect(page.getByTestId("value-visitors")).toHaveText("1");
  await expect(page.getByTestId("value-visits")).toHaveText("1");
  await expect(page.getByTestId("value-pageviews")).toHaveText("3");
  await expect(page.getByTestId("value-bounce_rate")).toHaveText("0%");
  await expect(page.getByTestId("realtime")).toContainText("1 current visitor");
  const sources = page.getByTestId("panel-sources");
  await expect(sources.getByRole("button", { name: "test", exact: true })).toBeVisible();
  const pages = page.getByTestId("panel-pages");
  for (const path of ["/demo.html", "/spa/about", "/spa/pricing"]) {
    await expect(pages.getByRole("button", { name: path, exact: true })).toBeVisible();
  }
  await expect(pages).not.toContainText("token");
  await page.screenshot({ path: `${shots}/dashboard.png`, fullPage: true });
  await page.emulateMedia({ colorScheme: "dark" });
  await page.waitForTimeout(400); // buttons fade their colors over 200 ms
  await page.screenshot({ path: `${shots}/dashboard-dark.png`, fullPage: true });
  await page.emulateMedia({ colorScheme: "light" });

  // Clicking a row filters; the chip removes the filter again. Both live in the URL.
  await pages.getByRole("button", { name: "/spa/about", exact: true }).click();
  await expect(page).toHaveURL(/f=page%3A%2Fspa%2Fabout/);
  await expect(page.getByRole("list", { name: "Filters" })).toContainText("/spa/about");
  await expect(page.getByTestId("value-pageviews")).toHaveText("1");
  await page.screenshot({ path: `${shots}/dashboard-filtered.png`, fullPage: true });
  await page.goBack();
  await expect(page.getByTestId("value-pageviews")).toHaveText("3");

  // Switching the chart's metric only rewrites the URL.
  await page.getByTestId("metric-pageviews").click();
  await expect(page).toHaveURL(/metric=pageviews/);
  await expect(page.getByTestId("metric-pageviews")).toHaveAttribute("aria-selected", "true");

  // The sites list shows the visitor and the settings page deletes the site.
  await page.goto("/");
  await expect(page.getByRole("link", { name: new RegExp(name) })).toContainText("1");
  await page.screenshot({ path: `${shots}/sites.png`, fullPage: true });
  await page.goto(`/sites/${siteId}/settings`);
  await page.getByLabel(`Type ${name} to confirm`).fill(name);
  await page.getByRole("button", { name: "Delete site" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: new RegExp(name) })).toHaveCount(0);
});

test("pages and API routes need a session", async ({ request }) => {
  const res = await request.get("/api/sites/abcdefghij/realtime", { maxRedirects: 0 });
  expect(res.status()).toBe(401);
  const page = await request.get("/sites/abcdefghij", { maxRedirects: 0 });
  expect(page.status()).toBe(307);
  expect(page.headers()["location"]).toContain("/login?next=%2Fsites%2Fabcdefghij");
});
