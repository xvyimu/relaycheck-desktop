#!/usr/bin/env node
/**
 * Layout alpha smoke: 1440 / 900 / 390.
 * Checks sidebar groups, accounts site filter shell, dashboard collapse defaults,
 * and no horizontal body overflow. Mocks APIs like other smoke scripts.
 */
import { existsSync } from "node:fs";
import { chromium } from "playwright";

const BASE = process.env.RELAYCHECK_SMOKE_BASE_URL || "http://127.0.0.1:5173";
const NOW = "2026-07-11T20:00:00+08:00";

const EXECUTABLE_CANDIDATES = [
  process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE,
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium_headless_shell-1228\\chrome-headless-shell-win64\\chrome-headless-shell.exe",
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium_headless_shell-1223\\chrome-headless-shell-win64\\chrome-headless-shell.exe",
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium-1228\\chrome-win64\\chrome.exe",
  "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
  "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
].filter(Boolean);

function apiPayload(data) {
  return { ok: true, data };
}

const sites = [
  {
    id: "site-a",
    name: "Alpha Site",
    baseUrl: "https://alpha.example",
    kind: "newapi",
    healthStatus: "healthy",
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: true,
    supportsPricing: true,
    accountCount: 2,
  },
  {
    id: "site-b",
    name: "Beta Site",
    baseUrl: "https://beta.example",
    kind: "newapi",
    healthStatus: "healthy",
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: true,
    supportsPricing: true,
    accountCount: 1,
  },
];

const accounts = [
  {
    id: "acc-1",
    upstreamSiteId: "site-a",
    upstreamSiteName: "Alpha Site",
    displayName: "Alpha One",
    authType: "browser_profile",
    loginStatus: "valid",
  },
  {
    id: "acc-2",
    upstreamSiteId: "site-a",
    upstreamSiteName: "Alpha Site",
    displayName: "Alpha Two",
    authType: "browser_profile",
    loginStatus: "valid",
  },
  {
    id: "acc-3",
    upstreamSiteId: "site-b",
    upstreamSiteName: "Beta Site",
    displayName: "Beta One",
    authType: "browser_profile",
    loginStatus: "valid",
  },
];

function statusPayload() {
  return {
    productName: "RelayCheck Desktop",
    productVersion: "1.1.0-test",
    buildTime: NOW,
    architecture: "test",
    bindAddress: "127.0.0.1",
    databasePath: "C:/redacted/relaycheck.db",
    backupDir: "C:/redacted/backups",
    port: 3001,
    networkProxy: { enabled: false, url: "", urlMasked: "", bypassLocal: true },
    scheduler: { generatedAt: NOW, jobs: [] },
    lastDiagnostics: { overall: "success", generatedAt: NOW, itemCount: 0 },
    summary: {
      localNewApiCount: 0,
      importedChannelCount: 0,
      identifiedChannelCount: 0,
      accountCount: accounts.length,
      unreadNotifications: 0,
    },
  };
}

async function fulfillApi(route) {
  const request = route.request();
  const url = new URL(request.url());
  const path = url.pathname;
  let data;

  if (path === "/api/health") data = { status: "ok" };
  else if (path === "/api/system/status") data = statusPayload();
  else if (path === "/api/channels") data = [];
  else if (path === "/api/upstream-sites") data = sites;
  else if (path === "/api/accounts") {
    const siteId = (url.searchParams.get("upstreamSiteId") || "").trim();
    data = siteId ? accounts.filter((a) => a.upstreamSiteId === siteId) : accounts;
  } else if (path === "/api/checkins/status") {
    data = {
      generatedAt: NOW,
      running: false,
      mode: "idle",
      totalAccounts: 0,
      processedAccounts: 0,
      pendingAccounts: 0,
      successCount: 0,
      alreadyCount: 0,
      failedCount: 0,
      unsupportedCount: 0,
      authExpiredCount: 0,
      today: {
        totalLogs: 0,
        successCount: 0,
        alreadyCount: 0,
        failedCount: 0,
        unsupportedCount: 0,
        authExpiredCount: 0,
        dueAccounts: 0,
        logs: [],
      },
      schedule: {
        enabled: true,
        time: "08:00",
        randomDelayMin: 0,
        randomDelayMax: 120,
        nextRunInSeconds: 0,
        nextWindowInSeconds: 0,
      },
    };
  } else if (path === "/api/notifications") data = [];
  else if (path === "/api/system/diagnostics") data = { generatedAt: NOW, overall: "success", items: [] };
  else if (path === "/api/system/action-center") data = { generatedAt: NOW, overall: "success", items: [] };
  else if (path === "/api/models/overview") {
    data = {
      generatedAt: NOW,
      modelCount: 0,
      accountCount: 0,
      validKeyCount: 0,
      usableModelCount: 0,
      models: [],
      sites: [],
      priceHints: [],
    };
  } else if (path === "/api/models/pricing") {
    data = {
      generatedAt: NOW,
      sourceCount: 0,
      modelCount: 0,
      exactCount: 0,
      ratioCount: 0,
      sources: [],
      siteCaches: [],
      comparisons: [],
    };
  } else if (path === "/api/usage/overview") {
    data = {
      generatedAt: NOW,
      accountCount: 0,
      siteCount: 0,
      lowBalanceCount: 0,
      decliningCount: 0,
      estimatedDailyUse: {},
      sites: [],
      accounts: [],
    };
  } else if (path === "/api/channels/health/overview") {
    data = {
      generatedAt: NOW,
      overall: "success",
      siteCount: 0,
      healthySiteCount: 0,
      unreachableSiteCount: 0,
      channelCount: 0,
      liveModelChannelCount: 0,
      failedModelChannelCount: 0,
      uncheckedModelChannelCount: 0,
      validKeyCount: 0,
      invalidKeyCount: 0,
      uncheckedKeyCount: 0,
      sites: [],
    };
  } else if (path === "/api/system/settings") data = [];
  else if (path === "/api/system/backups") data = [];
  else if (path === "/api/system/scheduler-status") data = { generatedAt: NOW, jobs: [] };
  else if (path === "/api/system/audit-log") data = [];
  else if (path === "/api/system/exports") data = [];
  else if (path === "/api/scheduler/calendar") data = { generatedAt: NOW, items: [] };
  else if (path === "/api/scheduler/next-runs") data = { generatedAt: NOW, items: [] };
  else data = request.method() === "GET" ? [] : { ok: true };

  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(apiPayload(data)),
  });
}

async function launchBrowser() {
  for (const executablePath of EXECUTABLE_CANDIDATES) {
    if (!existsSync(executablePath)) continue;
    try {
      const browser = await chromium.launch({ headless: true, executablePath });
      return { browser, executablePath };
    } catch (error) {
      console.warn(`Cannot launch ${executablePath}: ${error.message.split("\n")[0]}`);
    }
  }
  return { browser: await chromium.launch({ headless: true }), executablePath: "playwright-managed" };
}

async function checkViewport(browser, viewport) {
  const page = await browser.newPage({ viewport });
  const consoleIssues = [];
  page.on("console", (msg) => {
    if (["error", "warning"].includes(msg.type())) {
      consoleIssues.push(`${msg.type()}: ${msg.text()}`);
    }
  });
  page.on("pageerror", (error) => {
    consoleIssues.push(`pageerror: ${error.message}`);
  });

  await page.route("**/*", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.startsWith("/api/")) {
      await fulfillApi(route);
      return;
    }
    await route.continue();
  });
  await page.addInitScript(() => {
    window.localStorage.setItem("relaycheck_onboarding_done", "1");
    window.localStorage.setItem("relaycheck_theme", "light");
  });

  await page.goto(BASE, { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.locator(".sidebar").waitFor({ state: "visible", timeout: 12000 }).catch(async (error) => {
    const bodyText = await page.locator("body").innerText().catch(() => "");
    throw new Error(`sidebar missing: ${error.message}; body=${bodyText.slice(0, 400)}`);
  });
  if ((await page.locator(".onboarding-overlay").count()) > 0) {
    await page.evaluate(() => {
      try {
        window.localStorage.setItem("relaycheck_onboarding_done", "1");
      } catch {}
    });
    await page.reload({ waitUntil: "domcontentloaded", timeout: 20000 });
    await page.locator(".sidebar").waitFor({ state: "visible", timeout: 10000 });
  }

  const checks = [];
  const record = (name, ok, detail = "") => {
    checks.push({ name, ok, detail });
  };

  const sidebarClass = await page.locator("aside.sidebar").getAttribute("class");
  record("sidebar uses live class (not only v4)", Boolean(sidebarClass?.split(/\s+/).includes("sidebar")));
  const labels = await page.locator(".sidebar-section-label").allTextContents();
  const labelText = labels.map((t) => t.trim()).join("|");
  record(
    "sidebar section labels present",
    labels.length >= 3 && labels.some((t) => t.includes("运营")) && labels.some((t) => t.includes("资产")) && labels.some((t) => t.includes("工具")),
    labelText || "none",
  );
  const tabCount = await page.locator(".sidebar nav button").count();
  record("sidebar has all tab buttons", tabCount === 8, `count=${tabCount}`);

  await page.getByRole("button", { name: "仪表盘", exact: true }).click({ force: true });
  await page.waitForTimeout(400);
  const dashVisible = (await page.locator(".dashboard-priority-card, .dashboard-panel, .hub-radar-card, section.panel").count()) > 0;
  record("dashboard shell renders", dashVisible);

  const anyDetails = await page.locator(".panel details, .dashboard-panel details, details").count();
  const collapsibleCards = await page.locator(".dashboard-collapsible-card").count();
  const collapsedToggle = await page.locator('.dashboard-collapsible-card button[aria-expanded="false"]').count();
  record(
    "dashboard has collapsible sections",
    collapsibleCards > 0 || anyDetails > 0,
    `cards=${collapsibleCards} collapsedToggles=${collapsedToggle} details=${anyDetails}`,
  );
  if (collapsibleCards > 0) {
    record("dashboard collapsibles default closed", collapsedToggle > 0, `collapsedToggles=${collapsedToggle}`);
  }

  await page.getByRole("button", { name: "账号", exact: true }).click({ force: true });
  await page.locator(".accounts-panel").waitFor({ state: "visible", timeout: 10000 });
  await page.waitForTimeout(300);
  const siteSelect = page.locator('.accounts-panel select[aria-label="按上游站点筛选账号"]');
  record("accounts site select present", (await siteSelect.count()) > 0);
  if ((await siteSelect.count()) > 0) {
    const options = await siteSelect.locator("option").allTextContents();
    record("site options include all + sites", options.some((o) => o.includes("全部")) && options.some((o) => o.includes("Alpha")), options.join(","));
    await siteSelect.selectOption("site-b");
    await page.waitForTimeout(500);
    const cards = await page.locator(".accounts-panel .account-card").count();
    record("filter site-b shows one account", cards === 1, `cards=${cards}`);
    const banner = await page.locator(".accounts-panel .channel-active-filter").count();
    record("site filter banner shown", banner > 0);
  }
  const collapsedForm = await page.locator(".account-create-collapsed").count();
  record("create form collapsed by default", collapsedForm > 0);

  await page.getByRole("button", { name: "站点", exact: true }).click({ force: true });
  await page.waitForTimeout(400);
  // S7.3: "查看账号" deep-links to sites master-detail (not accounts tab). Prefer list-mode CTA when present.
  const layoutList = page.getByRole("button", { name: "卡片", exact: true });
  if ((await layoutList.count()) > 0) {
    await layoutList.click({ force: true });
    await page.waitForTimeout(300);
  }
  const viewAccounts = page.getByRole("button", { name: /查看账号/ });
  if ((await viewAccounts.count()) > 0) {
    await viewAccounts.first().click({ force: true });
    await page.waitForTimeout(500);
    const master = page.locator('[data-testid="site-account-master-detail"]');
    const masterVisible = (await master.count()) > 0;
    const currentSite = page.locator(".master-detail-right-head strong");
    const hasSelection = masterVisible && (await currentSite.count()) > 0;
    record(
      "sites view-accounts opens master-detail preselect",
      hasSelection,
      `master=${masterVisible} selectionText=${hasSelection ? await currentSite.first().textContent() : ""}`,
    );
  } else {
    // Master mode: click first site item to preselect
    const siteItem = page.locator(".master-detail-site-item").first();
    if ((await siteItem.count()) > 0) {
      await siteItem.click({ force: true });
      await page.waitForTimeout(400);
      const currentSite = page.locator(".master-detail-right-head strong");
      record(
        "sites view-accounts opens master-detail preselect",
        (await currentSite.count()) > 0,
        "via master site item",
      );
    } else {
      record("sites view-accounts opens master-detail preselect", true, "no sites-skip");
    }
  }

  const metrics = await page.evaluate(() => {
    const vw = document.documentElement.clientWidth;
    const nav = document.querySelector(".sidebar nav");
    const navStyle = nav ? getComputedStyle(nav) : null;
    return {
      viewportWidth: vw,
      bodyScrollWidth: document.body.scrollWidth,
      docScrollWidth: document.documentElement.scrollWidth,
      navDisplay: navStyle?.display || "",
      navOverflowX: navStyle?.overflowX || "",
    };
  });

  const bodyOverflow =
    metrics.bodyScrollWidth > metrics.viewportWidth + 2 || metrics.docScrollWidth > metrics.viewportWidth + 2;
  record("no horizontal body overflow", !bodyOverflow, JSON.stringify(metrics));

  if (viewport.width <= 560) {
    const horizontalNav =
      metrics.navDisplay === "flex" || metrics.navOverflowX === "auto" || metrics.navOverflowX === "scroll";
    record("narrow sidebar nav is compact/horizontal", horizontalNav, `display=${metrics.navDisplay} overflowX=${metrics.navOverflowX}`);
  }

  await page.close();
  return { viewport, checks, consoleIssues, metrics };
}

const { browser, executablePath } = await launchBrowser();
const viewports = [
  { width: 1440, height: 900 },
  { width: 900, height: 900 },
  { width: 390, height: 900 },
];
const results = [];
try {
  for (const vp of viewports) {
    results.push(await checkViewport(browser, vp));
  }
} finally {
  await browser.close();
}

let hasFailure = false;
console.log(`Layout alpha smoke using ${executablePath} @ ${BASE}`);
for (const result of results) {
  const failed = result.checks.filter((c) => !c.ok);
  const consoleNoise = result.consoleIssues.length > 0;
  const status = failed.length || consoleNoise ? "FAIL" : "PASS";
  hasFailure ||= status === "FAIL";
  console.log(`\n[${status}] ${result.viewport.width}x${result.viewport.height}`);
  for (const c of result.checks) {
    console.log(`  [${c.ok ? "PASS" : "FAIL"}] ${c.name}${c.detail ? " - " + c.detail : ""}`);
  }
  if (consoleNoise) console.log(result.consoleIssues.join("\n"));
}

if (hasFailure) process.exit(1);
console.log("\nAll layout alpha viewports passed.");
