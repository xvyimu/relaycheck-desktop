#!/usr/bin/env node

import { existsSync } from "node:fs";
import { chromium } from "playwright";

const BASE = process.env.RELAYCHECK_SMOKE_BASE_URL || "http://127.0.0.1:5173";
const NOW = "2026-07-03T09:30:00+08:00";
const LONG_SITE_NAME =
  "超长站点名称用于验证移动端排程预览不会横向溢出 - RelayCheck NewAPI Enterprise Cluster Alpha";

const EXECUTABLE_CANDIDATES = [
  process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE,
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium_headless_shell-1228\\chrome-headless-shell-win64\\chrome-headless-shell.exe",
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium_headless_shell-1223\\chrome-headless-shell-win64\\chrome-headless-shell.exe",
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium-1228\\chrome-win64\\chrome.exe",
  "C:\\Users\\yuanjia\\AppData\\Local\\ms-playwright\\chromium-1223\\chrome-win64\\chrome.exe",
  "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
  "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
].filter(Boolean);

function apiPayload(data) {
  return { ok: true, data };
}

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
      identifiedChannelCount: 1,
      accountCount: 2,
      unreadNotifications: 0,
    },
  };
}

const sites = [
  {
    id: "site-long",
    name: LONG_SITE_NAME,
    baseUrl: "https://very-long-site.example.com",
    kind: "newapi",
    healthStatus: "ok",
    supportsCheckin: true,
    supportsBalance: true,
    supportsModels: true,
    supportsPricing: true,
    accountCount: 2,
  },
];

const schedules = [
  {
    id: "site-long",
    upstreamSiteId: "site-long",
    siteName: LONG_SITE_NAME,
    enabled: true,
    checkinTime: "08:00",
    cronExpr: "0 8 * * *",
    skipDates: ["2026-07-05"],
    randomDelayMin: 10,
    randomDelayMax: 20,
    nextRunAt: "2026-07-04T08:15:00+08:00",
    createdAt: NOW,
    updatedAt: NOW,
  },
];

const calendar = {
  generatedAt: NOW,
  items: [
    {
      date: "2026-07-04",
      time: "08:15",
      siteName: LONG_SITE_NAME,
      siteId: "site-long",
      jobType: "checkin",
      enabled: true,
    },
    {
      date: "2026-07-05",
      time: "09:30",
      siteName: "全局同步任务",
      siteId: "__sync__",
      jobType: "sync",
      enabled: true,
    },
  ],
};

const nextRuns = {
  generatedAt: NOW,
  items: [
    {
      jobKey: "channel.site-long",
      label: `${LONG_SITE_NAME} 签到(0 8 * * *)`,
      nextRunAt: "2026-07-04T08:15:00+08:00",
      nextRunInSeconds: 81900,
      status: "scheduled",
      siteId: "site-long",
      siteName: LONG_SITE_NAME,
    },
  ],
};

async function fulfillApi(route) {
  const request = route.request();
  const path = new URL(request.url()).pathname;
  let data;

  if (path === "/api/health") data = { status: "ok" };
  else if (path === "/api/system/status") data = statusPayload();
  else if (path === "/api/channels") data = [];
  else if (path === "/api/upstream-sites") data = sites;
  else if (path === "/api/accounts") data = [];
  else if (path === "/api/checkins/status") {
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
  } else if (path === "/api/system/settings") {
    data = [
      {
        key: "network.proxy",
        valueJson: JSON.stringify({ enabled: false, url: "", bypassLocal: true }),
        updatedAt: NOW,
      },
      {
        key: "sync.schedule",
        valueJson: JSON.stringify({
          enabled: true,
          intervalMinutes: 30,
          mode: "local-newapi",
          runOnStartup: false,
        }),
        updatedAt: NOW,
      },
      {
        key: "channel.health.schedule",
        valueJson: JSON.stringify({
          enabled: true,
          intervalMinutes: 60,
          runOnStartup: false,
          limit: 20,
          onlyRisky: false,
        }),
        updatedAt: NOW,
      },
      { key: "app.version_check_url", valueJson: JSON.stringify(""), updatedAt: NOW },
    ];
  } else if (path === "/api/system/backups") data = [];
  else if (path === "/api/system/scheduler-status") data = { generatedAt: NOW, jobs: [] };
  else if (path === "/api/system/audit-log") data = [];
  else if (path === "/api/system/exports") data = [];
  else if (path === "/api/scheduler/channel-schedules") data = schedules;
  else if (path === "/api/scheduler/calendar") data = calendar;
  else if (path === "/api/scheduler/next-runs") data = nextRuns;
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
  const navButtons = page.locator(".sidebar nav button");
  await navButtons.first().waitFor({ state: "visible", timeout: 10000 }).catch(async (error) => {
    const bodyText = await page.locator("body").innerText().catch(() => "");
    const html = await page.content().catch(() => "");
    throw new Error(
      [
        `sidebar navigation did not render: ${error.message}`,
        `console: ${consoleIssues.join(" | ") || "none"}`,
        `body: ${bodyText.slice(0, 500) || "(empty)"}`,
        `html: ${html.slice(0, 500) || "(empty)"}`,
      ].join("\n"),
    );
  });
  const settingsButton = page.getByRole("button", { name: "设置", exact: true });
  await settingsButton.click({ force: true });
  await page.locator(".site-schedules-card").waitFor({ state: "visible", timeout: 10000 }).catch(async (error) => {
    const bodyText = await page.locator("body").innerText().catch(() => "");
    throw new Error(
      [
        `site schedules card did not render: ${error.message}`,
        `console: ${consoleIssues.join(" | ") || "none"}`,
        `body: ${bodyText.slice(0, 800) || "(empty)"}`,
      ].join("\n"),
    );
  });
  await page.locator(".calendar-preview-row").first().waitFor({ state: "visible", timeout: 10000 });
  await page.locator(".next-run-row").first().waitFor({ state: "visible", timeout: 10000 });
  await page.waitForTimeout(300);

  const metrics = await page.evaluate(() => {
    const rows = [...document.querySelectorAll(".calendar-preview-row, .next-run-row")];
    const viewportWidth = document.documentElement.clientWidth;
    const rowIssues = rows
      .map((row) => {
        const rect = row.getBoundingClientRect();
        return {
          className: row.className,
          left: Math.round(rect.left),
          right: Math.round(rect.right),
          width: Math.round(rect.width),
          scrollWidth: row.scrollWidth,
          clientWidth: row.clientWidth,
          outsideViewport: rect.left < -1 || rect.right > viewportWidth + 1,
          internalOverflow: row.scrollWidth > row.clientWidth + 1,
        };
      })
      .filter((row) => row.outsideViewport || row.internalOverflow);

    return {
      viewportWidth,
      bodyScrollWidth: document.body.scrollWidth,
      docScrollWidth: document.documentElement.scrollWidth,
      rowIssues,
    };
  });

  await page.close();
  return { viewport, metrics, consoleIssues };
}

const { browser, executablePath } = await launchBrowser();
const results = [];

try {
  results.push(await checkViewport(browser, { width: 1440, height: 900 }));
  results.push(await checkViewport(browser, { width: 390, height: 900 }));
} finally {
  await browser.close();
}

let hasFailure = false;
console.log(`Schedule settings layout smoke using ${executablePath}`);
for (const result of results) {
  const bodyOverflow =
    result.metrics.bodyScrollWidth > result.metrics.viewportWidth + 1 ||
    result.metrics.docScrollWidth > result.metrics.viewportWidth + 1;
  const rowOverflow = result.metrics.rowIssues.length > 0;
  const consoleNoise = result.consoleIssues.length > 0;
  const status = bodyOverflow || rowOverflow || consoleNoise ? "FAIL" : "PASS";
  hasFailure ||= status === "FAIL";
  console.log(
    `[${status}] ${result.viewport.width}x${result.viewport.height} bodyOverflow=${bodyOverflow} rowOverflow=${rowOverflow} consoleIssues=${result.consoleIssues.length}`,
  );
  if (rowOverflow) console.log(JSON.stringify(result.metrics.rowIssues, null, 2));
  if (consoleNoise) console.log(result.consoleIssues.join("\n"));
}

if (hasFailure) {
  process.exit(1);
}
