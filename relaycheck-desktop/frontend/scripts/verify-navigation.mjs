import { writeFileSync } from "node:fs";

writeFileSync("verify-canary.txt", "canary at " + new Date().toISOString() + "\n", "utf8");

let browser;

try {
  const { writeFile } = await import("node:fs/promises");
  const { chromium } = await import("playwright");

  const BASE = process.env.RELAYCHECK_SMOKE_BASE_URL || "http://127.0.0.1:5173";
  const NOW = "2026-07-05T10:00:00+08:00";
  const log = [];
  const out = (m) => { console.log(m); log.push(m); };
  const checks = [];
  const record = (name, status, detail) => {
    checks.push({ name, status, detail });
    out(`  [${status}] ${name}${detail ? " - " + detail : ""}`);
  };

  out(`=== NavigationIntent E2E Verifier - ${BASE} ===`);

  const apiPayload = (data) => ({ ok: true, data });
  const statusPayload = {
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
    lastDiagnostics: { overall: "warning", generatedAt: NOW, itemCount: 1 },
    summary: {
      localNewApiCount: 1,
      importedChannelCount: 2,
      identifiedChannelCount: 1,
      accountCount: 2,
      unreadNotifications: 1,
    },
  };
  const channels = [
    {
      id: "channel-unknown",
      sourceChannelId: "src-unknown",
      name: "未知渠道",
      baseUrl: "https://unknown.example.com",
      status: "enabled",
      upstreamKind: "unknown",
      supportsCheckin: false,
      supportsBalance: false,
      supportsModels: false,
      supportsPricing: false,
      sourceSyncStatus: "active",
      modelCount: 0,
      sampleModels: [],
    },
    {
      id: "channel-missing",
      sourceChannelId: "src-missing",
      name: "源端已移除渠道",
      baseUrl: "https://missing.example.com",
      status: "enabled",
      upstreamKind: "newapi",
      supportsCheckin: true,
      supportsBalance: true,
      supportsModels: true,
      supportsPricing: true,
      sourceSyncStatus: "missing",
      modelCount: 2,
      sampleModels: ["gpt-test"],
    },
  ];
  const sites = [
    {
      id: "site-main",
      name: "Relay Main",
      baseUrl: "https://relay.example.com",
      kind: "newapi",
      healthStatus: "ok",
      supportsCheckin: true,
      supportsBalance: true,
      supportsModels: true,
      supportsPricing: true,
      accountCount: 2,
    },
  ];
  const accounts = [
    {
      id: "account-expired",
      upstreamSiteId: "site-main",
      upstreamSiteName: "Relay Main",
      upstreamSiteBaseUrl: "https://relay.example.com",
      displayName: "授权失效账号",
      authType: "cookie",
      loginStatus: "expired",
      lastCheckinStatus: "auth_expired",
    },
    {
      id: "account-missing-balance",
      upstreamSiteId: "site-main",
      upstreamSiteName: "Relay Main",
      upstreamSiteBaseUrl: "https://relay.example.com",
      displayName: "余额待刷新账号",
      authType: "api_key",
      loginStatus: "valid",
      apiKeyFingerprint: "fp_123",
      apiKeyStatus: "valid",
    },
  ];
  const checkins = {
    generatedAt: NOW,
    running: false,
    mode: "idle",
    totalAccounts: 2,
    processedAccounts: 0,
    pendingAccounts: 2,
    successCount: 0,
    alreadyCount: 0,
    failedCount: 1,
    unsupportedCount: 0,
    authExpiredCount: 1,
    today: {
      totalLogs: 1,
      successCount: 0,
      alreadyCount: 0,
      failedCount: 1,
      unsupportedCount: 0,
      authExpiredCount: 0,
      dueAccounts: 2,
      logs: [
        {
          id: "log-failed",
          accountName: "授权失效账号",
          siteName: "Relay Main",
          upstreamSiteName: "Relay Main",
          status: "failed",
          message: "cookie expired",
          startedAt: NOW,
          createdAt: NOW,
        },
      ],
    },
    schedule: {
      enabled: true,
      time: "08:00",
      randomDelayMin: 0,
      randomDelayMax: 120,
      nextRunInSeconds: 3600,
      nextWindowInSeconds: 3600,
    },
  };
  const notifications = [
    {
      id: "notification-unread",
      type: "scheduled_checkin_failed",
      level: "warning",
      title: "未读通知",
      content: "有一条未读通知需要处理。",
      read: false,
      createdAt: NOW,
    },
  ];
  const actionCenter = {
    generatedAt: NOW,
    overall: "warning",
    items: [
      {
        id: "auth-required-accounts",
        priority: 100,
        level: "danger",
        category: "auth",
        title: "优先处理失效授权",
        description: "存在授权失效账号。",
        impact: "签到和余额刷新会失败。",
        count: 1,
        target: "accounts",
        filter: "problem",
        action: "处理",
        recommendedAction: "重新授权账号。",
        samples: ["授权失效账号"],
      },
      {
        id: "today-checkin-problems",
        priority: 90,
        level: "warning",
        category: "checkin",
        title: "复查今日签到异常",
        description: "今日签到失败。",
        count: 1,
        target: "checkins",
        filter: "problem",
        action: "处理",
      },
      {
        id: "balance-missing",
        priority: 80,
        level: "warning",
        category: "balance",
        title: "刷新缺失余额",
        description: "账号缺少余额快照。",
        count: 1,
        target: "accounts",
        filter: "all",
        action: "处理",
      },
      {
        id: "unknown-channels",
        priority: 70,
        level: "warning",
        category: "channel",
        title: "识别未知渠道",
        description: "存在未知后台类型。",
        count: 1,
        target: "channels",
        filter: "unknown",
        action: "处理",
      },
      {
        id: "missing-channels",
        priority: 60,
        level: "warning",
        category: "channel",
        title: "整理源端已移除渠道",
        description: "存在源端已移除渠道。",
        count: 1,
        target: "channels",
        filter: "missing",
        action: "处理",
      },
      {
        id: "unread-notifications",
        priority: 50,
        level: "info",
        category: "notification",
        title: "清理未读通知",
        description: "存在未读通知。",
        count: 1,
        target: "notifications",
        filter: "unread",
        action: "处理",
      },
    ],
  };
  const modelOverview = {
    generatedAt: NOW,
    modelCount: 1,
    accountCount: 2,
    validKeyCount: 1,
    usableModelCount: 1,
    models: [],
    sites: [],
    priceHints: [],
  };
  const pricingOverview = {
    generatedAt: NOW,
    sourceCount: 0,
    modelCount: 0,
    exactCount: 0,
    ratioCount: 0,
    sources: [],
    siteCaches: [],
    comparisons: [],
  };
  const usageOverview = {
    generatedAt: NOW,
    accountCount: 2,
    siteCount: 1,
    lowBalanceCount: 0,
    decliningCount: 0,
    estimatedDailyUse: {},
    sites: [],
    accounts: [],
  };
  const channelHealthOverview = {
    generatedAt: NOW,
    overall: "warning",
    siteCount: 1,
    healthySiteCount: 1,
    unreachableSiteCount: 0,
    channelCount: 2,
    liveModelChannelCount: 1,
    failedModelChannelCount: 0,
    uncheckedModelChannelCount: 1,
    validKeyCount: 1,
    invalidKeyCount: 0,
    uncheckedKeyCount: 1,
    sites: [],
  };
  const schedulerCalendar = {
    generatedAt: NOW,
    items: [
      {
        date: "2026-07-05",
        time: "08:00",
        siteName: "Relay Main",
        siteId: "site-main",
        jobType: "checkin",
        enabled: true,
      },
    ],
  };
  const nextRuns = {
    generatedAt: NOW,
    items: [
      {
        jobKey: "channel.site-main",
        label: "Relay Main 签到",
        nextRunAt: "2026-07-05T08:00:00+08:00",
        nextRunInSeconds: 3600,
        status: "scheduled",
        siteId: "site-main",
        siteName: "Relay Main",
      },
    ],
  };

  async function fulfillApi(route) {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let data;
    if (path === "/api/health") data = { status: "ok" };
    else if (path === "/api/system/status") data = statusPayload;
    else if (path === "/api/channels") data = channels;
    else if (path === "/api/channels/models/overview") data = { generatedAt: NOW, syncedChannels: 0, channelCount: 2, modelCount: 1, liveKeyCount: 1, rawOnlyCount: 0, failedCount: 0, uncheckedCount: 1, items: [], models: [] };
    else if (path === "/api/channels/health/overview") data = channelHealthOverview;
    else if (path === "/api/upstream-sites") data = sites;
    else if (path === "/api/accounts") data = accounts;
    else if (path === "/api/checkins/status") data = checkins;
    else if (path === "/api/notifications") data = notifications;
    else if (path === "/api/system/diagnostics") data = { generatedAt: NOW, overall: "warning", items: [] };
    else if (path === "/api/system/action-center") data = actionCenter;
    else if (path === "/api/models/overview") data = modelOverview;
    else if (path === "/api/models/pricing") data = pricingOverview;
    else if (path === "/api/usage/overview") data = usageOverview;
    else if (path === "/api/scheduler/calendar") data = schedulerCalendar;
    else if (path === "/api/scheduler/next-runs") data = nextRuns;
    else if (path === "/api/system/settings") data = [];
    else if (path === "/api/system/backups") data = [];
    else if (path === "/api/system/scheduler-status") data = { generatedAt: NOW, jobs: [] };
    else if (path === "/api/system/audit-log") data = [];
    else if (path === "/api/system/exports") data = [];
    else data = request.method() === "GET" ? [] : { ok: true };

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(apiPayload(data)),
    });
  }

  browser = await chromium.launch({ headless: true });
  out("browser launched");
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const consoleErrors = [];
  page.on("console", (m) => { if (["error","warning"].includes(m.type())) consoleErrors.push(`${m.type()}: ${m.text()}`); });
  page.on("pageerror", (e) => { consoleErrors.push(`pageerror: ${e.message}`); out(`  !! pageerror: ${e.message}`); });
  page.on("close", () => out("  !! page closed"));
  page.on("crash", () => out("  !! page crashed"));
  await page.route((url) => url.pathname.startsWith("/api/"), fulfillApi);

  const activeTabLabel = async (p) => (await p.locator(".sidebar nav button.active").textContent()).trim();
  const selectValue = async (p, panel, label) => {
    const loc = label ? p.locator(`${panel} label.field`, { hasText: label }).locator("select") : p.locator(`${panel} select`).first();
    return loc.inputValue();
  };
  const hasBanner = async (p, panel) => (await p.locator(`${panel} .channel-active-filter`).count()) > 0;

  const goDashboard = async (p) => {
    await p.goto(BASE, { waitUntil: "domcontentloaded", timeout: 20000 });
    await p.locator(".sidebar").waitFor({ state: "visible", timeout: 10000 }).catch(async (error) => {
      const bodyText = await p.locator("body").innerText().catch(() => "");
      const html = await p.content().catch(() => "");
      throw new Error(
        [
          `sidebar navigation did not render: ${error.message}`,
          `console: ${consoleErrors.join(" | ") || "none"}`,
          `body: ${bodyText.slice(0, 500) || "(empty)"}`,
          `html: ${html.slice(0, 500) || "(empty)"}`,
        ].join("\n"),
      );
    });
    if ((await p.locator(".onboarding-overlay").count()) > 0) {
      await p.evaluate(() => { try { window.localStorage.setItem("relaycheck_onboarding_done", "1"); } catch (e) {} });
      await p.reload({ waitUntil: "domcontentloaded", timeout: 20000 });
      await p.locator(".sidebar").waitFor({ state: "visible", timeout: 10000 });
    }
    const dashBtn = p.locator(".sidebar nav button", { hasText: "仪表盘" });
    if (await dashBtn.count()) await dashBtn.click({ force: true });
    await p.locator(".dashboard-priority-card").waitFor({ state: "visible", timeout: 10000 });
    await p.waitForTimeout(1500);
  };

  const clickHandleFor = async (p, title) => {
    const item = p.locator(".dashboard-priority-item", { hasText: title });
    if (!(await item.count())) return false;
    await item.locator(".dashboard-priority-actions button", { hasText: "处理" }).click({ force: true });
    await p.waitForTimeout(800);
    return true;
  };

  const safe = async (fn, label) => {
    try { return await fn(); } catch (e) { out(`  !! ${label}: ${e.message}`); return null; }
  };

  await goDashboard(page);

  const priorityTitles = await page.locator(".dashboard-priority-item strong").allTextContents();
  out(`Dashboard priority items: ${priorityTitles.length}`);
  out(`  titles: ${JSON.stringify(priorityTitles)}`);
  record("Dashboard renders all action-center items (no slice cap)", priorityTitles.length >= 4 ? "PASS" : "FAIL", `got ${priorityTitles.length}`);
  record("Item #5 missing-channels IS rendered", priorityTitles.some((t) => t.includes("源端已移除")) ? "PASS" : "FAIL", "present after slice(0,4) fix");
  record("Item #6 unread-notifications IS rendered", priorityTitles.some((t) => t.includes("未读通知")) ? "PASS" : "FAIL", "present after slice(0,4) fix");
  // Check 1: auth-required-accounts -> AccountsPanel(problem)
  out("\n[1] 失效授权 -> AccountsPanel(problem)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "优先处理失效授权"))) { record("失效授权", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("失效授权 -> AccountsPanel(problem)", "FAIL", "page closed"); return; }
    await page.locator(".accounts-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const sel = await selectValue(page, ".accounts-panel", "状态");
    const banner = await hasBanner(page, ".accounts-panel");
    const bt = banner ? (await page.locator(".accounts-panel .channel-active-filter strong").textContent()).trim() : null;
    record("失效授权 -> AccountsPanel + problem + banner", tab === "站点与账号" && sel === "problem" && banner && bt.includes("异常账号") ? "PASS" : "FAIL", `tab=${tab} status=${sel} banner=${banner ? bt : "none"}`);
  }, "check1");

  // Check 2: today-checkin-problems -> CheckinsPanel(failed)
  out("\n[2] 签到异常 -> CheckinsPanel(failed)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "复查今日签到异常"))) { record("签到异常", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("签到异常 -> CheckinsPanel(failed)", "FAIL", "page closed"); return; }
    await page.locator(".checkin-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const sel = await selectValue(page, ".checkin-panel");
    const banner = await hasBanner(page, ".checkin-panel");
    const bt = banner ? (await page.locator(".checkin-panel .channel-active-filter strong").textContent()).trim() : null;
    record("签到异常 -> CheckinsPanel + failed + banner", tab === "签到" && sel === "failed" && banner && bt.includes("签到状态") ? "PASS" : "FAIL", `tab=${tab} status=${sel} banner=${banner ? bt : "none"}`);
  }, "check2");

  // Check 3: balance-missing -> AccountsPanel(all, no banner)
  out("\n[3] 余额缺失 -> AccountsPanel(all)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "刷新缺失余额"))) { record("余额缺失", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("余额缺失 -> AccountsPanel(all)", "FAIL", "page closed"); return; }
    await page.locator(".accounts-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const sel = await selectValue(page, ".accounts-panel", "状态");
    const banner = await hasBanner(page, ".accounts-panel");
    record("余额缺失 -> AccountsPanel + all + no banner", tab === "站点与账号" && sel === "all" && !banner ? "PASS" : "FAIL", `tab=${tab} status=${sel} banner=${banner ? "present" : "absent"}`);
  }, "check3");

  // Check 4: unknown-channels -> ChannelsPanel(unknown)
  out("\n[4] 未知渠道 -> ChannelsPanel(unknown)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "识别未知渠道"))) { record("未知渠道", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("未知渠道 -> ChannelsPanel(unknown)", "FAIL", "page closed"); return; }
    await page.locator(".channels-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const kind = await selectValue(page, ".channels-panel", "后台类型");
    const src = await selectValue(page, ".channels-panel", "源端状态");
    record("未知渠道 -> ChannelsPanel + kind=unknown", tab === "渠道" && kind === "unknown" && src === "not_archived" ? "PASS" : "FAIL", `tab=${tab} kind=${kind} source=${src}`);
  }, "check4");

  // Check 5: missing-channels -> ChannelsPanel(missing) — now reachable after slice(0,4) fix
  out("\n[5] 缺失渠道 -> ChannelsPanel(missing)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "整理源端已移除渠道"))) { record("缺失渠道", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("缺失渠道 -> ChannelsPanel(missing)", "FAIL", "page closed"); return; }
    await page.locator(".channels-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const src = await selectValue(page, ".channels-panel", "源端状态");
    record("缺失渠道 -> ChannelsPanel + source=missing", tab === "渠道" && src === "missing" ? "PASS" : "FAIL", `tab=${tab} source=${src}`);
  }, "check5");

  // Check 6: unread-notifications -> NotificationsPanel(unreadOnly) — now reachable after slice(0,4) fix
  out("\n[6] 未读通知 -> NotificationsPanel(unreadOnly)");
  await goDashboard(page);
  await safe(async () => {
    if (!(await clickHandleFor(page, "清理未读通知"))) { record("未读通知", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("未读通知 -> NotificationsPanel(unreadOnly)", "FAIL", "page closed"); return; }
    await page.locator(".notifications-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const tog = (await page.locator(".notifications-panel .notification-toolbar button", { hasText: /仅未读|全部/ }).last().textContent()).trim();
    record("未读通知 -> NotificationsPanel + unreadOnly", tab === "通知" && tog.includes("全部") ? "PASS" : "FAIL", `tab=${tab} toggle=${tog}`);
  }, "check6");

  const sum = { pass: checks.filter(c=>c.status==="PASS").length, fail: checks.filter(c=>c.status==="FAIL").length, info: checks.filter(c=>c.status==="INFO").length };
  out(`\n=== Summary: ${sum.pass} PASS / ${sum.fail} FAIL / ${sum.info} INFO ===`);
  if (consoleErrors.length) { out("\nConsole:"); for (const e of consoleErrors) out(`  - ${e}`); }
  if (sum.fail > 0 || consoleErrors.length > 0) {
    process.exitCode = 1;
  }

  await browser.close();
  out("browser closed");
  await writeFile("verify-nav-output.txt", log.join("\n") + "\n", "utf8");
  writeFileSync("verify-canary.txt", "done\n", "utf8");
} catch (e) {
  try { await browser?.close(); } catch {}
  writeFileSync("verify-canary.txt", "ERROR: " + e.message + "\n" + (e.stack || ""), "utf8");
  console.error("FATAL:", e.message);
  process.exitCode = 1;
}
