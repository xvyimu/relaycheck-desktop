import { chromium } from "playwright";

const BASE = process.env.RELAYCHECK_SMOKE_BASE_URL || "http://127.0.0.1:5173";
const NOW = "2026-07-18T01:00:00Z";

const checkins = {
  generatedAt: NOW,
  running: false,
  mode: "idle",
  totalAccounts: 2,
  processedAccounts: 0,
  pendingAccounts: 2,
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
    dueAccounts: 2,
    logs: [],
  },
  schedule: {
    enabled: true,
    time: "08:00",
    randomDelayMin: 0,
    randomDelayMax: 120,
    nextRunInSeconds: 3600,
    nextWindowInSeconds: 7200,
  },
};

const state = {
  dryRunMode: "success",
  dryRunFailuresRemaining: 0,
  scanMode: "success",
  startMode: "success",
  dryRunCount: 0,
  startCount: 0,
  cleanupMode: "success",
  cleanupPreviewCount: 0,
  cleanupConfirmCount: 0,
  cleanupConfirmBodies: [],
  order: [],
};

const cleanupAccount = {
  id: "account-cleanup",
  upstreamSiteId: "site-cleanup",
  upstreamSiteName: "Cleanup Relay",
  upstreamSiteBaseUrl: "https://cleanup.example",
  displayName: "待清理账号",
  authType: "cookie",
  loginStatus: "valid",
  lastCheckinStatus: "unsupported",
};

/** 包装与真实后端一致的成功 JSON envelope。 */
function ok(data) {
  return { ok: true, data };
}

/** 生成批量签到 dry-run 夹具。 */
function preview() {
  if (state.dryRunMode === "zero") {
    return {
      type: "checkin",
      maxAccounts: 200,
      totalAccounts: 1,
      willRun: 0,
      skipped: 1,
      items: [
        {
          accountId: "account-missing",
          accountName: "待补凭据账号",
          siteName: "Relay A",
          action: "skip_missing_credentials",
          reason: "缺少 Cookie、令牌、API Key 或登录密码",
        },
      ],
    };
  }
  return {
    type: "checkin",
    previewId: `preview-${state.dryRunCount}`,
    expiresAt: "2026-07-18T01:05:00Z",
    maxAccounts: 200,
    totalAccounts: 2,
    willRun: 1,
    skipped: 1,
    items: [
      {
        accountId: "account-ready",
        accountName: "已就绪账号",
        siteName: "Relay A",
        action: "will_run",
        reason: "本地认证条件已就绪，将尝试签到",
      },
      {
        accountId: "account-missing",
        accountName: "待补凭据账号",
        siteName: "Relay A",
        action: "skip_missing_credentials",
        reason: "缺少 Cookie、令牌、API Key 或登录密码",
      },
    ],
  };
}

/** 生成账号清理候选夹具，确保 previewId 与页面展示的账号一一对应。 */
function cleanupPreview() {
  return {
    previewId: `cleanup-preview-${state.cleanupPreviewCount}`,
    expiresAt: "2026-07-18T01:05:00Z",
    matched: 1,
    deleted: 0,
    limit: 10,
    hasMore: false,
    dryRun: true,
    includeLastUnsupported: true,
    items: [
      {
        accountId: cleanupAccount.id,
        accountName: cleanupAccount.displayName,
        upstreamSiteId: cleanupAccount.upstreamSiteId,
        upstreamSiteName: cleanupAccount.upstreamSiteName,
        upstreamSiteKind: "oneapi",
        lastCheckinStatus: "unsupported",
        reason: "last_checkin_unsupported",
      },
    ],
  };
}

/** 根据当前模式生成本机扫描成功、混合或失败结果。 */
function scanResult() {
  const success = {
    dbPath: "C:/redacted/one-api.db",
    baseUrl: "http://127.0.0.1:3000",
    importedCount: 2,
    sitesCreated: 1,
    sitesMerged: 0,
  };
  if (state.scanMode === "failure") {
    return {
      found: true,
      message: "扫描完成，但导入失败。",
      results: [{ ...success, importedCount: 0, sitesCreated: 0, error: "数据库不可读" }],
    };
  }
  if (state.scanMode === "mixed") {
    return {
      found: true,
      message: "部分数据库已导入，另有项目需要处理。",
      results: [
        success,
        { ...success, dbPath: "C:/redacted/broken.db", importedCount: 0, sitesCreated: 0, error: "数据库不可读" },
      ],
    };
  }
  return { found: true, message: "扫描并导入完成。", results: [success] };
}

/** 以确定的状态码和 JSON 内容完成 Playwright 路由。 */
async function fulfillJSON(route, data, status = 200) {
  await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(data) });
}

/** 根据请求方法和路径返回确定性 API 夹具，并记录安全流程的调用顺序。 */
async function handleAPI(route) {
  const request = route.request();
  const path = new URL(request.url()).pathname;

  if (path === "/api/tasks/dry-run") {
    state.dryRunCount += 1;
    state.order.push("dry-run");
    if (state.dryRunFailuresRemaining > 0) {
      state.dryRunFailuresRemaining -= 1;
      await fulfillJSON(route, { ok: false, error: "预览服务暂时不可用", errorClass: "server_error" }, 500);
      return;
    }
    await fulfillJSON(route, ok(preview()));
    return;
  }
  if (path === "/api/tasks/start") {
    state.startCount += 1;
    state.order.push("start");
    if (state.startMode === "conflict") {
      await fulfillJSON(
        route,
        { ok: false, error: "签到预览已过期或已使用，请重新预览。", errorClass: "conflict" },
        409,
      );
      return;
    }
    await fulfillJSON(route, ok({ taskId: `task-${state.startCount}` }));
    return;
  }
  if (/^\/api\/tasks\/[^/]+\/stream$/.test(path)) {
    state.order.push("stream");
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: `data: ${JSON.stringify({ id: "task-1", type: "checkin", status: "done", current: 1, total: 1, results: [], startedAt: NOW, updatedAt: NOW })}\n\n`,
    });
    return;
  }
  if (path === "/api/local-newapi/auto-detect-import") {
    await fulfillJSON(route, ok(scanResult()));
    return;
  }
  if (path === "/api/accounts/delete-unsupported-checkins") {
    const body = request.postDataJSON();
    if (body.dryRun === true) {
      state.cleanupPreviewCount += 1;
      state.order.push("cleanup-preview");
      await fulfillJSON(route, ok(cleanupPreview()));
      return;
    }
    state.cleanupConfirmCount += 1;
    state.cleanupConfirmBodies.push(body);
    state.order.push("cleanup-confirm");
    if (state.cleanupMode === "conflict") {
      await fulfillJSON(
        route,
        { ok: false, error: "清理预览已过期、已使用或候选状态已变化，请重新预览。", errorClass: "conflict" },
        409,
      );
      return;
    }
    if (state.cleanupMode === "error") {
      await fulfillJSON(
        route,
        { ok: false, error: "账号清理失败，请重新预览后重试。", errorClass: "server_error" },
        500,
      );
      return;
    }
    await fulfillJSON(
      route,
      ok({
        matched: 1,
        deleted: 1,
        limit: 10,
        hasMore: false,
        dryRun: false,
        includeLastUnsupported: true,
        items: cleanupPreview().items,
      }),
    );
    return;
  }

  const payloads = new Map([
    ["/api/health", { status: "ok" }],
    [
      "/api/system/status",
      {
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
        lastDiagnostics: { overall: "ok", generatedAt: NOW, itemCount: 0 },
        summary: {
          localNewApiCount: 0,
          importedChannelCount: 0,
          identifiedChannelCount: 0,
          accountCount: 0,
          unreadNotifications: 0,
        },
      },
    ],
    ["/api/dashboard/inventory", { channels: [], sites: [], accountSummary: { accountTotal: 1, problemTotal: 1 } }],
    [
      "/api/dashboard/ops",
      {
        checkins,
        notifications: { items: [], total: 0, unreadTotal: 0, importantTotal: 0, nextOffset: null },
        diagnostics: { generatedAt: NOW, overall: "ok", items: [] },
        actionCenter: { generatedAt: NOW, overall: "ok", items: [] },
      },
    ],
    [
      "/api/dashboard/model-usage",
      {
        model: {
          generatedAt: NOW,
          modelCount: 0,
          accountCount: 0,
          validKeyCount: 0,
          usableModelCount: 0,
          models: [],
          sites: [],
          priceHints: [],
        },
        pricing: {
          generatedAt: NOW,
          sourceCount: 0,
          modelCount: 0,
          exactCount: 0,
          ratioCount: 0,
          sources: [],
          siteCaches: [],
          comparisons: [],
        },
        usage: {
          generatedAt: NOW,
          accountCount: 0,
          siteCount: 0,
          lowBalanceCount: 0,
          decliningCount: 0,
          estimatedDailyUse: {},
          sites: [],
          accounts: [],
        },
      },
    ],
    ["/api/local-newapi", []],
    ["/api/local-newapi/exclude-rules", { rules: [], note: "" }],
    ["/api/channels", []],
    [
      "/api/channels/models/overview",
      {
        generatedAt: NOW,
        syncedChannels: 0,
        channelCount: 0,
        modelCount: 0,
        liveKeyCount: 0,
        rawOnlyCount: 0,
        failedCount: 0,
        uncheckedCount: 0,
        items: [],
        models: [],
      },
    ],
    [
      "/api/channels/health/overview",
      {
        generatedAt: NOW,
        overall: "ok",
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
      },
    ],
    ["/api/upstream-sites", []],
    ["/api/accounts/page", { items: [cleanupAccount], total: 1, accountTotal: 1, problemTotal: 1 }],
    ["/api/accounts/summary", { accountTotal: 1, problemTotal: 1 }],
    ["/api/accounts/search-sites", { items: [], truncated: false }],
  ]);
  const data = payloads.has(path) ? payloads.get(path) : request.method() === "GET" ? [] : {};
  await fulfillJSON(route, ok(data));
}

/** 在 smoke 断言失败时抛出带上下文的错误。 */
function assert(condition, message) {
  if (!condition) throw new Error(message);
}

/** 返回当前激活的侧边栏标签文本。 */
async function activeTab(page) {
  return (await page.locator(".sidebar nav button.active").textContent())?.trim() || "";
}

/** 通过用户可见标签切换主导航。 */
async function goTab(page, label) {
  await page.locator(".sidebar nav button", { hasText: label }).click();
}

let browser;
try {
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() !== "error") return;
    const text = message.text();
    if (
      text === "Failed to load resource: the server responded with a status of 500 (Internal Server Error)" ||
      text === "Failed to load resource: the server responded with a status of 409 (Conflict)"
    ) {
      return;
    }
    consoleErrors.push(text);
  });
  page.on("pageerror", (error) => consoleErrors.push(error.message));
  await page.route((url) => url.pathname.startsWith("/api/"), handleAPI);

  await page.goto(BASE, { waitUntil: "domcontentloaded", timeout: 20_000 });
  await page
    .locator(".sidebar")
    .waitFor({ state: "visible", timeout: 10_000 })
    .catch(async (error) => {
      const body = await page
        .locator("body")
        .innerText()
        .catch(() => "");
      throw new Error(
        `sidebar did not render: ${error.message}\nbody=${body.slice(0, 800)}\nconsole=${consoleErrors.join(" | ") || "none"}`,
      );
    });
  await page.evaluate(() => window.localStorage.setItem("relaycheck_onboarding_done", "1"));
  await page.waitForTimeout(200);
  if (
    await page
      .locator(".onboarding-card")
      .isVisible()
      .catch(() => false)
  ) {
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.locator(".sidebar").waitFor({ state: "visible", timeout: 10_000 });
  }

  await goTab(page, "签到");
  const runAll = page.getByRole("button", { name: "执行全部签到" });
  state.order = [];
  const startsBeforeConfirm = state.startCount;
  await runAll.click();
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor();
  assert(state.startCount === startsBeforeConfirm, "task start happened before confirmation");
  const streamRequest = page.waitForRequest((request) =>
    /\/api\/tasks\/[^/]+\/stream$/.test(new URL(request.url()).pathname),
  );
  await page.getByRole("button", { name: "确认执行" }).click();
  await streamRequest;
  await page.waitForTimeout(50);
  assert(state.startCount === startsBeforeConfirm + 1, "confirmed preview did not start exactly one task");
  assert(
    state.order.slice(0, 3).join(" -> ") === "dry-run -> start -> stream",
    `unexpected request order: ${state.order.join(" -> ")}`,
  );

  const startsBeforeCancel = state.startCount;
  await runAll.click();
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  assert(state.startCount === startsBeforeCancel, "cancelling preview started a task");

  state.dryRunMode = "zero";
  await runAll.click();
  await page.getByText("没有可执行账号").waitFor();
  assert(await page.getByRole("button", { name: "确认执行" }).isDisabled(), "zero-runnable confirmation was enabled");
  await page.getByRole("button", { name: "取消", exact: true }).click();

  state.dryRunMode = "success";
  state.dryRunFailuresRemaining = 1;
  await runAll.click();
  await page.getByRole("alert").filter({ hasText: "预览服务暂时不可用" }).waitFor();
  const startsBeforeRetry = state.startCount;
  await page.getByRole("button", { name: "重新预览" }).click();
  await page.getByText("已就绪账号").waitFor();
  assert(state.startCount === startsBeforeRetry, "retrying dry-run started a task");
  await page.getByRole("button", { name: "取消", exact: true }).click();

  state.startMode = "conflict";
  await runAll.click();
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor();
  await page.getByRole("button", { name: "确认执行" }).click();
  await page.getByRole("alert").filter({ hasText: "预览可能已失效" }).waitFor();
  assert(
    await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).isVisible(),
    "409 closed the recoverable preview dialog",
  );
  await page.getByRole("button", { name: "关闭签到预览" }).click();
  state.startMode = "success";

  await goTab(page, "站点与账号");
  await page.getByRole("button", { name: "全部账号", exact: true }).click();
  await page.getByRole("button", { name: "展开洞察" }).click();
  const cleanupPanel = page.locator(".unsupported-cleanup-panel");
  await cleanupPanel.getByRole("button", { name: "预览清理" }).click();
  await cleanupPanel.getByText("待清理账号").waitFor();

  const confirmsBeforeCancel = state.cleanupConfirmCount;
  page.once("dialog", async (dialog) => dialog.dismiss());
  await cleanupPanel.getByRole("button", { name: "删除本批" }).click();
  assert(state.cleanupConfirmCount === confirmsBeforeCancel, "cancelling cleanup confirmation sent a delete request");

  const acceptedPreviewId = `cleanup-preview-${state.cleanupPreviewCount}`;
  page.once("dialog", async (dialog) => dialog.accept());
  await cleanupPanel.getByRole("button", { name: "删除本批" }).click();
  await page.getByText("已删除 1 个不支持签到账号").waitFor();
  assert(
    state.cleanupConfirmCount === confirmsBeforeCancel + 1,
    "cleanup confirmation did not send exactly one request",
  );
  assert(
    JSON.stringify(state.cleanupConfirmBodies.at(-1)) === JSON.stringify({ previewId: acceptedPreviewId }),
    `cleanup confirmation changed the frozen contract: ${JSON.stringify(state.cleanupConfirmBodies.at(-1))}`,
  );

  state.cleanupMode = "conflict";
  await cleanupPanel.getByRole("button", { name: "再次检查" }).click();
  await cleanupPanel.getByText("待清理账号").waitFor();
  page.once("dialog", async (dialog) => dialog.accept());
  await cleanupPanel.getByRole("button", { name: "删除本批" }).click();
  await page.getByText("请重新预览后再确认").waitFor();
  assert(
    await cleanupPanel.getByRole("button", { name: "删除本批" }).isDisabled(),
    "409 left the stale cleanup preview enabled",
  );
  state.cleanupMode = "error";
  await cleanupPanel.getByRole("button", { name: "预览清理" }).click();
  await cleanupPanel.getByText("待清理账号").waitFor();
  page.once("dialog", async (dialog) => dialog.accept());
  await cleanupPanel.getByRole("button", { name: "删除本批" }).click();
  await page.getByText("当前预览已失效，请重新预览后再确认").waitFor();
  assert(
    await cleanupPanel.getByRole("button", { name: "删除本批" }).isDisabled(),
    "500 left the consumed cleanup preview enabled",
  );
  state.cleanupMode = "success";

  await page.evaluate(() => window.localStorage.removeItem("relaycheck_onboarding_done"));
  await page.reload({ waitUntil: "domcontentloaded" });
  const onboarding = page.getByRole("dialog", { name: "首次启动引导" });
  await onboarding.waitFor();
  assert(
    !(await onboarding.textContent()).includes("左侧账号页"),
    "onboarding still contains the retired account-page copy",
  );
  for (let step = 0; step < 3; step += 1) {
    await onboarding.getByRole("button", { name: "跳过" }).click();
  }
  await onboarding.getByText("步骤 4/4").waitFor();
  const startsBeforeOnboarding = state.startCount;
  await onboarding.getByRole("button", { name: "前往安全预览" }).click();
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor();
  assert(state.startCount === startsBeforeOnboarding, "onboarding bypassed preview confirmation");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.evaluate(() => window.dispatchEvent(new CustomEvent("relaycheck:reopen-onboarding")));
  await onboarding.getByText("步骤 1/4").waitFor();
  await page.keyboard.press("Escape");

  await goTab(page, "本机扫描");
  const scan = page.getByRole("button", { name: "检测并导入本机 NewAPI 数据库" });
  state.scanMode = "success";
  await scan.click();
  await page.getByRole("button", { name: "查看渠道" }).waitFor();
  await page.getByRole("button", { name: "查看渠道" }).click();
  assert((await activeTab(page)) === "渠道", "successful scan did not navigate to channels");

  await goTab(page, "本机扫描");
  state.scanMode = "mixed";
  await scan.click();
  await page.getByText("部分数据库已导入").waitFor();
  await page.getByRole("button", { name: "前往站点与账号" }).click();
  assert((await activeTab(page)) === "站点与账号", "mixed scan did not navigate to sites/accounts");

  await goTab(page, "本机扫描");
  state.scanMode = "failure";
  await scan.click();
  await page.getByText("扫描完成，但导入失败").waitFor();
  assert(
    (await page.getByRole("button", { name: "查看渠道" }).count()) === 0,
    "failed scan exposed success navigation",
  );

  await goTab(page, "签到");
  await page.setViewportSize({ width: 390, height: 900 });
  await runAll.focus();
  await runAll.click();
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  assert(overflow <= 0, `390px viewport has horizontal overflow: ${overflow}px`);
  for (const button of await page.locator(".checkin-preview-actions button").all()) {
    const box = await button.boundingBox();
    assert(Boolean(box && box.height >= 44), "preview action target is shorter than 44px");
  }
  await page.keyboard.press("Escape");
  await page.getByRole("dialog", { name: /确认将尝试执行的账号/ }).waitFor({ state: "hidden" });
  await page.waitForTimeout(50);
  assert(
    (await page.evaluate(() => document.activeElement?.textContent || "")).includes("执行全部签到"),
    "dialog did not restore focus",
  );

  assert(consoleErrors.length === 0, `browser console errors: ${consoleErrors.join(" | ")}`);
  console.log(
    `Incremental flows passed at ${BASE}: dryRun=${state.dryRunCount}, starts=${state.startCount}, cleanupPreviews=${state.cleanupPreviewCount}, cleanupConfirms=${state.cleanupConfirmCount}`,
  );
} finally {
  await browser?.close();
}
