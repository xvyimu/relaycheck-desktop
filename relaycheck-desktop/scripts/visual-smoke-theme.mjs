/**
 * Headless light/dark + nav visual smoke against a running RelayCheck UI.
 * Usage: node scripts/visual-smoke-theme.mjs --base http://127.0.0.1:3015 --out .tmp/visual-smoke
 *
 * Resolves playwright from frontend/node_modules (not a root dependency).
 */
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", "frontend", "package.json"));
const { chromium } = require("playwright");
const args = process.argv.slice(2);
function arg(name, fallback = "") {
  const i = args.indexOf(`--${name}`);
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback;
}

const base = arg("base", "http://127.0.0.1:3001").replace(/\/$/, "");
const outDir = path.resolve(arg("out", path.join(__dirname, "..", ".tmp", "visual-smoke")));
fs.mkdirSync(outDir, { recursive: true });

const failures = [];
function ok(name) {
  console.log(`[PASS] ${name}`);
}
function fail(name, detail) {
  console.log(`[FAIL] ${name}: ${detail}`);
  failures.push(`${name}: ${detail}`);
}

async function waitHealth(timeoutMs = 45000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${base}/api/health`);
      if (res.ok) return;
    } catch {
      // retry
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`health not ready at ${base}/api/health`);
}

async function shot(page, name) {
  const file = path.join(outDir, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  return file;
}

async function forceTheme(page, mode) {
  await page.evaluate((m) => {
    localStorage.setItem("relaycheck-theme", m);
    const root = document.documentElement;
    if (m === "dark") {
      root.classList.add("dark");
      root.style.colorScheme = "dark";
    } else {
      root.classList.remove("dark");
      root.style.colorScheme = "light";
    }
  }, mode);
  await page.waitForTimeout(200);
}

function bgLuminance(rgb) {
  // rgb(r, g, b) or rgba
  const m = String(rgb).match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/i);
  if (!m) return null;
  const [r, g, b] = m.slice(1, 4).map(Number);
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
}

async function main() {
  console.log(`Visual smoke base=${base}`);
  console.log(`Screenshots → ${outDir}`);
  await waitHealth();

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  const consoleErrors = [];
  page.on("pageerror", (err) => consoleErrors.push(String(err)));
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });

  await page.goto(base + "/", { waitUntil: "networkidle", timeout: 60000 });
  await page.waitForSelector(".sidebar, .brand", { timeout: 30000 });

  // Dismiss onboarding if present (Escape / close)
  const closeOnboarding = page.locator(
    '.onboarding-card button:has-text("跳过"), .onboarding-card button:has-text("稍后"), [aria-label*="关闭"]',
  );
  if (await closeOnboarding.first().isVisible().catch(() => false)) {
    await closeOnboarding.first().click().catch(() => {});
  }
  // force-hide overlay if still blocking
  await page.evaluate(() => {
    document.querySelectorAll(".onboarding-overlay, .onboarding-card").forEach((el) => {
      el.style.display = "none";
    });
  });

  // Light theme
  await forceTheme(page, "light");
  const lightHasDark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
  if (lightHasDark) fail("light class", "html still has .dark");
  else ok("light: html without .dark");
  // Surfaces are often transparent + gradient; sample token + a card/chip solid.
  async function sampleSurface() {
    return page.evaluate(() => {
      const root = getComputedStyle(document.documentElement);
      const token =
        root.getPropertyValue("--surface-solid").trim() ||
        root.getPropertyValue("--v4-card").trim() ||
        root.getPropertyValue("--surface").trim();
      const el =
        document.querySelector(".card") ||
        document.querySelector(".metric-card") ||
        document.querySelector(".brand") ||
        document.querySelector("main") ||
        document.body;
      const bg = getComputedStyle(el).backgroundColor;
      return { token, bg, tag: el.tagName + (el.className ? "." + String(el.className).split(" ")[0] : "") };
    });
  }

  function parseColorToL(value) {
    if (!value) return null;
    const hex = String(value).trim();
    if (hex.startsWith("#") && (hex.length === 7 || hex.length === 4)) {
      let r, g, b;
      if (hex.length === 7) {
        r = parseInt(hex.slice(1, 3), 16);
        g = parseInt(hex.slice(3, 5), 16);
        b = parseInt(hex.slice(5, 7), 16);
      } else {
        r = parseInt(hex[1] + hex[1], 16);
        g = parseInt(hex[2] + hex[2], 16);
        b = parseInt(hex[3] + hex[3], 16);
      }
      return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
    }
    return bgLuminance(value);
  }

  const lightSurf = await sampleSurface();
  const lightL = parseColorToL(lightSurf.token) ?? bgLuminance(lightSurf.bg);
  if (lightL == null || lightL < 0.45)
    fail("light surface", `token=${lightSurf.token} bg=${lightSurf.bg} on ${lightSurf.tag} L=${lightL}`);
  else ok(`light surface L=${lightL.toFixed(3)} token=${lightSurf.token || lightSurf.bg}`);
  await shot(page, "01-dashboard-light");

  // Dark theme
  await forceTheme(page, "dark");
  const darkHasDark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
  if (!darkHasDark) fail("dark class", "html missing .dark");
  else ok("dark: html has .dark");
  const darkColorScheme = await page.evaluate(() => document.documentElement.style.colorScheme);
  if (darkColorScheme !== "dark") fail("colorScheme", `got ${darkColorScheme}`);
  else ok("dark colorScheme=dark");
  const darkSurf = await sampleSurface();
  const darkL = parseColorToL(darkSurf.token) ?? bgLuminance(darkSurf.bg);
  if (darkL == null) fail("dark surface", `token=${darkSurf.token} bg=${darkSurf.bg}`);
  else if (lightL != null && darkL >= lightL - 0.08)
    fail("dark surface", `L=${darkL} not darker than light L=${lightL} token=${darkSurf.token}`);
  else ok(`dark surface L=${darkL.toFixed(3)} token=${darkSurf.token || darkSurf.bg}`);
  await shot(page, "02-dashboard-dark");

  // Theme toggle control present
  const toggle = page.locator("button.theme-toggle, .theme-toggle, .topbar-actions button").first();
  if (await toggle.count()) ok("theme control present");
  else fail("theme control", "not found in topbar");

  // Navigate key tabs (dark)
  const tabs = [
    { label: "渠道", file: "03-channels-dark" },
    { label: "站点与账号", file: "04-sites-dark" },
    { label: "签到", file: "05-checkins-dark" },
    { label: "本机扫描", file: "06-scan-dark" },
    { label: "设置", file: "07-settings-dark" },
    { label: "仪表盘", file: "08-dashboard-dark-return" },
  ];
  for (const t of tabs) {
    const btn = page.locator(`nav button:has-text("${t.label}")`).first();
    if (!(await btn.count())) {
      fail(`nav ${t.label}`, "button missing");
      continue;
    }
    await btn.click();
    await page.waitForTimeout(400);
    // no blank root
    const textLen = await page.evaluate(() => document.body.innerText.length);
    if (textLen < 40) fail(`nav ${t.label} content`, `innerText length ${textLen}`);
    else ok(`nav ${t.label} rendered (${textLen} chars)`);
    await shot(page, t.file);
  }

  // Light sites tab (drawer chrome if any empty state still ok)
  await forceTheme(page, "light");
  await page.locator('nav button:has-text("站点与账号")').first().click();
  await page.waitForTimeout(400);
  await shot(page, "09-sites-light");

  // Dialog shell: open onboarding if button exists, else skip
  const startGuide = page.locator('button:has-text("引导"), button:has-text("新手")').first();
  if (await startGuide.isVisible().catch(() => false)) {
    await startGuide.click();
    await page.waitForTimeout(300);
    const dialog = page.locator('[role="dialog"]');
    if (await dialog.count()) {
      ok("dialog shell opened");
      await shot(page, "10-dialog-light");
      await page.keyboard.press("Escape");
      await page.waitForTimeout(200);
      if (await dialog.isVisible().catch(() => false)) fail("dialog Escape", "still visible");
      else ok("dialog closed on Escape");
    }
  } else {
    ok("dialog open skipped (no guide CTA on empty install)");
  }

  // Surface critical console errors (filter noise)
  const hardErrors = consoleErrors.filter(
    (e) =>
      !/favicon|Download the React DevTools|net::ERR_|Content Security Policy directive/i.test(e),
  );
  if (hardErrors.length) fail("console", hardErrors.slice(0, 5).join(" | "));
  else ok("no hard page console errors");

  await browser.close();

  const report = {
    base,
    outDir,
    failures,
    screenshots: fs.readdirSync(outDir).filter((f) => f.endsWith(".png")),
    at: new Date().toISOString(),
  };
  fs.writeFileSync(path.join(outDir, "report.json"), JSON.stringify(report, null, 2));

  if (failures.length) {
    console.error(`\nVisual smoke FAILED (${failures.length})`);
    process.exit(1);
  }
  console.log("\nVisual smoke PASSED");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
