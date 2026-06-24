import { access, mkdir } from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright";

const baseUrl = process.env.SMOKE_BASE_URL || "http://127.0.0.1:3001";
const username = process.env.RELAYCHECK_SMOKE_USER || "admin";
const password = process.env.RELAYCHECK_SMOKE_PASSWORD;
const outDir = path.resolve(process.cwd(), "..", ".pipeline", "test-results");

if (!password) {
  throw new Error("Set RELAYCHECK_SMOKE_PASSWORD before running npm run smoke.");
}

async function exists(filePath) {
  try {
    await access(filePath);
    return true;
  } catch {
    return false;
  }
}

async function findBrowser() {
  const candidates = [
    process.env.PLAYWRIGHT_CHROME_PATH,
    "C:/Program Files/Google/Chrome/Application/chrome.exe",
    "C:/Program Files (x86)/Google/Chrome/Application/chrome.exe",
    "C:/Program Files/Microsoft/Edge/Application/msedge.exe",
    "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (await exists(candidate)) return candidate;
  }
  return undefined;
}

function assertNoOverflow(result, label) {
  if (result.widthOverflow) {
    throw new Error(`${label} has horizontal overflow: ${result.scrollWidth}px > ${result.clientWidth}px`);
  }
}

async function captureState(page, label, options = {}) {
  const minCards = options.minCards ?? 1;
  const result = await page.evaluate(() => ({
    hasRelayCheck: document.body.innerText.includes("RelayCheck"),
    active: document.querySelector(".sidebar nav button.active")?.textContent?.trim() || "",
    cards: document.querySelectorAll(
      ".card, .metric-card, .channel-card, .account-card, .site-card, .checkin-card, .notification-card, .channel-summary > div, .channels-panel, .accounts-panel, .sites-panel, .checkin-panel, .notifications-panel, .settings-grid",
    ).length,
    scrollWidth: document.body.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
    widthOverflow: document.body.scrollWidth > document.documentElement.clientWidth + 2,
  }));

  if (!result.hasRelayCheck) throw new Error(`${label} did not render RelayCheck text.`);
  if (!result.active) throw new Error(`${label} has no active navigation item.`);
  if (result.cards < minCards) throw new Error(`${label} rendered too few cards: ${result.cards}.`);
  assertNoOverflow(result, label);
  return result;
}

async function openTab(page, name, label = name) {
  await page.getByRole("button", { name }).click();
  await page.locator(".topbar h1", { hasText: name }).waitFor({
    state: "visible",
    timeout: 10000,
  });
  await page.waitForTimeout(500);
  const requiredSelectors = {
    Accounts: ".accounts-panel",
    Channels: ".channels-panel",
    Sites: ".sites-panel",
    "Check-ins": ".checkin-panel",
    Notifications: ".notifications-panel",
    Settings: ".settings-grid",
  };
  const selector = requiredSelectors[name];
  if (selector && !(await page.locator(selector).count())) {
    throw new Error(`${label} did not render expected selector: ${selector}`);
  }
  if (name === "Accounts" && !(await page.locator(".unsupported-cleanup-panel").count())) {
    throw new Error(`${label} did not render unsupported check-in cleanup panel.`);
  }
  return captureState(page, label, { minCards: 1 });
}

await mkdir(outDir, { recursive: true });

const executablePath = await findBrowser();
const browser = await chromium.launch({
  headless: true,
  ...(executablePath ? { executablePath } : {}),
});

const consoleErrors = [];
const pageErrors = [];

try {
  const page = await browser.newPage({
    viewport: { width: 1365, height: 768 },
    deviceScaleFactor: 1,
  });
  page.on("console", (msg) => {
    if (["error", "warning"].includes(msg.type())) {
      consoleErrors.push(`${msg.type()}: ${msg.text()}`);
    }
  });
  page.on("pageerror", (error) => pageErrors.push(error.message));

  await page.goto(baseUrl, { waitUntil: "networkidle", timeout: 15000 });
  if (await page.locator("form.login-card").count()) {
    await page.locator('input[autocomplete="username"]').fill(username);
    await page.locator('input[autocomplete="current-password"]').fill(password);
    await page.getByRole("button", { name: /log in/i }).click();
  }

  await page.getByRole("heading", { name: "Dashboard" }).waitFor({
    state: "visible",
    timeout: 10000,
  });

  const desktopPath = path.join(outDir, "app-shell-desktop-smoke.png");
  await page.screenshot({ path: desktopPath, fullPage: true });
  const desktop = await captureState(page, "desktop", { minCards: 3 });
  const tabs = {};
  for (const tabName of ["Channels", "Sites", "Accounts", "Check-ins", "Notifications", "Settings", "Dashboard"]) {
    tabs[tabName] = await openTab(page, tabName);
  }
  const settingsPath = path.join(outDir, "app-shell-settings-smoke.png");
  await page.getByRole("button", { name: "Settings" }).click();
  await page.locator(".topbar h1", { hasText: "Settings" }).waitFor({ state: "visible", timeout: 10000 });
  await page.waitForTimeout(500);
  await page.screenshot({ path: settingsPath, fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  const mobilePath = path.join(outDir, "app-shell-mobile-smoke.png");
  await page.screenshot({ path: mobilePath, fullPage: true });
  const mobile = await captureState(page, "mobile");
  const mobileTabs = {};
  for (const tabName of ["Channels", "Sites", "Accounts", "Check-ins", "Notifications", "Settings", "Dashboard"]) {
    mobileTabs[tabName] = await openTab(page, tabName, `mobile ${tabName}`);
  }

  if (consoleErrors.length || pageErrors.length) {
    throw new Error(`Browser errors detected: ${JSON.stringify({ consoleErrors, pageErrors })}`);
  }

  console.log(JSON.stringify({
    ok: true,
    baseUrl,
    browser: executablePath || "playwright-managed",
    desktop,
    tabs,
    mobile,
    mobileTabs,
    screenshots: [desktopPath, settingsPath, mobilePath],
  }, null, 2));
} finally {
  await browser.close();
}
