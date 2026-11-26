// Renders docs/assets/banner.html into the README's header and footer banners (light and dark,
// 2x):
//
//   node scripts/banner.mjs   # from web/; needs network access for the Google font
import { chromium } from "@playwright/test";

const assets = new URL("../../docs/assets/", import.meta.url);
const browser = await chromium.launch();
for (const theme of ["light", "dark"]) {
  const page = await browser.newPage({
    deviceScaleFactor: 2,
    viewport: { width: 1320, height: 680 },
  });
  await page.goto(new URL("banner.html", assets).href);
  await page.evaluate((t) => document.documentElement.setAttribute("data-theme", t), theme);
  await page.evaluate(() => document.fonts.ready);
  for (const [id, name] of [
    ["#banner", "banner"],
    ["#footer", "footer"],
  ]) {
    await page.locator(id).screenshot({
      path: new URL(`${name}-${theme}.png`, assets).pathname,
      omitBackground: true,
    });
  }
  await page.close();
}
await browser.close();
console.log(`banners in ${assets.pathname}`);
