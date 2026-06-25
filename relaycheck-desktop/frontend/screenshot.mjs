import { chromium } from "../node_modules/playwright/index.mjs";

const BASE = "http://127.0.0.1:3001";
const TABS = [
  { key: "dashboard", label: "仪表盘" },
  { key: "channels", label: "渠道" },
  { key: "sites", label: "站点" },
  { key: "accounts", label: "账号" },
  { key: "checkins", label: "签到" },
  { key: "notifications", label: "通知" },
  { key: "settings", label: "设置" },
];
const VIEWPORTS = [
  { name: "desktop", width: 1440, height: 900 },
  { name: "mobile", width: 390, height: 844 },
];

async function main() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ deviceScaleFactor: 2 });

  for (const vp of VIEWPORTS) {
    const page = await context.newPage();
    await page.setViewportSize({ width: vp.width, height: vp.height });

    for (const tab of TABS) {
      await page.goto(BASE, { waitUntil: "networkidle" });
      await page.waitForSelector(".sidebar", { timeout: 5000 }).catch(() => {});

      const tabBtn = page.locator(".sidebar nav button", { hasText: tab.label });
      if (await tabBtn.isVisible()) {
        await tabBtn.click();
        await page.waitForTimeout(1200);
      }

      await page.screenshot({
        path: `../screenshots/${vp.name}-${tab.key}.png`,
        fullPage: true,
      });

      if (tab.key === "notifications") {
        await page.screenshot({
          path: `../screenshots/${vp.name}-${tab.key}-viewport.png`,
          fullPage: false,
        });
      }

      console.log(`  ✓ ${vp.name}/${tab.key}`);
    }
    await page.close();
  }

  await browser.close();
  console.log("Done — all screenshots in screenshots/");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});