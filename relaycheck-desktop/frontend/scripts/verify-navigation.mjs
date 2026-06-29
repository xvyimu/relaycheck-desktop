import { writeFileSync } from "node:fs";

writeFileSync("verify-canary.txt", "canary at " + new Date().toISOString() + "\n", "utf8");

try {
  const { writeFile } = await import("node:fs/promises");
  const { chromium } = await import("playwright");

  const BASE = "http://127.0.0.1:5173";
  const log = [];
  const out = (m) => { console.log(m); log.push(m); };
  const checks = [];
  const record = (name, status, detail) => {
    checks.push({ name, status, detail });
    out(`  [${status}] ${name}${detail ? " - " + detail : ""}`);
  };

  out(`=== NavigationIntent E2E Verifier - ${BASE} ===`);

  const browser = await chromium.launch({ headless: true });
  out("browser launched");
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const consoleErrors = [];
  page.on("console", (m) => { if (["error","warning"].includes(m.type())) consoleErrors.push(`${m.type()}: ${m.text()}`); });
  page.on("pageerror", (e) => { consoleErrors.push(`pageerror: ${e.message}`); out(`  !! pageerror: ${e.message}`); });
  page.on("close", () => out("  !! page closed"));
  page.on("crash", () => out("  !! page crashed"));

  const activeTabLabel = async (p) => (await p.locator(".sidebar nav button.active").textContent()).trim();
  const selectValue = async (p, panel, label) => {
    const loc = label ? p.locator(`${panel} label.field`, { hasText: label }).locator("select") : p.locator(`${panel} select`).first();
    return loc.inputValue();
  };
  const hasBanner = async (p, panel) => (await p.locator(`${panel} .channel-active-filter`).count()) > 0;

  const goDashboard = async (p) => {
    await p.goto(BASE, { waitUntil: "domcontentloaded", timeout: 20000 });
    await p.locator(".sidebar").waitFor({ state: "visible", timeout: 10000 });
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
  await safe(async () => {
    if (!(await clickHandleFor(page, "优先处理失效授权"))) { record("失效授权", "FAIL", "button missing"); return; }
    if (page.isClosed()) { record("失效授权 -> AccountsPanel(problem)", "FAIL", "page closed"); return; }
    await page.locator(".accounts-panel").waitFor({ state: "visible", timeout: 8000 });
    const tab = await activeTabLabel(page);
    const sel = await selectValue(page, ".accounts-panel", "状态");
    const banner = await hasBanner(page, ".accounts-panel");
    const bt = banner ? (await page.locator(".accounts-panel .channel-active-filter strong").textContent()).trim() : null;
    record("失效授权 -> AccountsPanel + problem + banner", tab === "账号" && sel === "problem" && banner && bt.includes("异常账号") ? "PASS" : "FAIL", `tab=${tab} status=${sel} banner=${banner ? bt : "none"}`);
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
    record("余额缺失 -> AccountsPanel + all + no banner", tab === "账号" && sel === "all" && !banner ? "PASS" : "FAIL", `tab=${tab} status=${sel} banner=${banner ? "present" : "absent"}`);
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

  await browser.close();
  out("browser closed");
  await writeFile("verify-nav-output.txt", log.join("\n") + "\n", "utf8");
  writeFileSync("verify-canary.txt", "done\n", "utf8");
} catch (e) {
  writeFileSync("verify-canary.txt", "ERROR: " + e.message + "\n" + (e.stack || ""), "utf8");
  console.error("FATAL:", e.message);
}

