import { useEffect, useState } from "react";
import { api } from "@/api/client";
import { formatPriceComparisonBadge, formatPriceComparisonMeta, formatPricingSource } from "@/lib/format";
import { apiKeyStatusLabel, formatAPIKeyTestMessage, priceLevelLabel, priceLevelShort, pricingCacheStatusLabel, pricingSourceBadge } from "@/lib/labels";
import type { Account, APIKeyTestResult, BulkBrowserOpenResponse, BulkBrowserSaveResponse, BulkPasswordLoginResponse, KeyExportPreview, ModelOverview, ModelPricingOverview, UnsupportedCheckinCleanupResult } from "@/types";
import { EmptyState } from "@/components/ui/empty-state";
import { isLocalURL } from "@/components/accounts/helpers";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { TaskProgressView } from "@/components/ui/TaskProgressView";

const API_KEY_STALE_MS = 24 * 60 * 60 * 1000;
const UNSUPPORTED_CLEANUP_LIMIT = 10;
const LABELS_TEST_KEYS = { title: "批量测试 Key" } as const;
const LABELS_REFRESH_BALANCE = { title: "批量刷新余额" } as const;

function isStaleAPIKeyCheck(account: Account) {
  if (!account.apiKeyFingerprint) return false;
  if (!account.apiKeyLastCheckedAt) return true;
  const checkedAt = new Date(account.apiKeyLastCheckedAt).getTime();
  if (!Number.isFinite(checkedAt)) return true;
  return Date.now() - checkedAt > API_KEY_STALE_MS;
}

function uniqueAccounts(accounts: Account[]) {
  const seen = new Set<string>();
  return accounts.filter((account) => {
    if (seen.has(account.id)) return false;
    seen.add(account.id);
    return true;
  });
}

function buildModelCoverage(accounts: Account[]) {
  const grouped = new Map<string, { model: string; accountIds: Set<string>; siteSamples: Set<string> }>();
  for (const account of accounts) {
    const models = new Set([...(account.apiKeySampleModels || []), account.apiKeyTestModel || ""].map((model) => model.trim()).filter(Boolean));
    for (const model of models) {
      const current = grouped.get(model) || { model, accountIds: new Set<string>(), siteSamples: new Set<string>() };
      current.accountIds.add(account.id);
      if (account.upstreamSiteName) current.siteSamples.add(account.upstreamSiteName);
      grouped.set(model, current);
    }
  }
  return Array.from(grouped.values())
    .map((item) => ({
      model: item.model,
      accountCount: item.accountIds.size,
      siteSamples: Array.from(item.siteSamples).slice(0, 3),
    }))
    .sort((left, right) => right.accountCount - left.accountCount || left.model.localeCompare(right.model));
}

function cleanupReasonLabel(reason: string) {
  switch (reason) {
    case "site_not_support_checkin":
      return "站点不支持签到";
    case "last_checkin_unsupported":
      return "上次签到不支持";
    default:
      return reason || "不支持签到";
  }
}
function keyIssueLabel(account: Account) {
  if (account.apiKeyStatus && !["valid", "unchecked"].includes(account.apiKeyStatus)) {
    return apiKeyStatusLabel(account.apiKeyStatus);
  }
  if (!account.apiKeyLastCheckedAt || account.apiKeyStatus === "unchecked") {
    return "未检测";
  }
  if (isStaleAPIKeyCheck(account)) {
    return "超过 24 小时未重测";
  }
  return apiKeyStatusLabel(account.apiKeyStatus || "unchecked");
}

function downloadJSON(fileName: string, body: string) {
  const blob = new Blob([body], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function AccountInsights({ accounts, onDone, onModelFilter }: { accounts: Account[]; onDone: () => void; onModelFilter?: (model: string) => void }) {
  const pendingAuth = accounts.filter((account) => account.lastCheckinStatus === "auth_expired");
  const localSuspects = accounts.filter((account) => isLocalURL(account.upstreamSiteBaseUrl || ""));
  const successful = accounts.filter((account) => account.lastCheckinStatus === "success");
  const unsupportedCheckinAccounts = accounts.filter((account) => account.lastCheckinStatus === "unsupported");
  const keyAccounts = accounts.filter((account) => account.apiKeyFingerprint);
  const validKeyAccounts = keyAccounts.filter((account) => account.apiKeyStatus === "valid");
  const problemKeyAccounts = keyAccounts.filter((account) => account.apiKeyStatus && !["valid", "unchecked"].includes(account.apiKeyStatus));
  const uncheckedKeyAccounts = keyAccounts.filter((account) => !account.apiKeyStatus || account.apiKeyStatus === "unchecked");
  const staleKeyAccounts = keyAccounts.filter(isStaleAPIKeyCheck);
  const usableModelAccounts = keyAccounts.filter((account) => account.apiKeyModelUsable);
  const totalKnownModels = keyAccounts.reduce((sum, account) => sum + (account.apiKeyModelCount || 0), 0);
  const latencyValues = keyAccounts.map((account) => account.apiKeyLatencyMs || 0).filter((value) => value > 0);
  const averageLatency = latencyValues.length ? Math.round(latencyValues.reduce((sum, value) => sum + value, 0) / latencyValues.length) : 0;
  const modelSamples = Array.from(new Set(keyAccounts.flatMap((account) => account.apiKeySampleModels || []))).slice(0, 6);
  const speedRankAccounts = keyAccounts
    .filter((account) => (account.apiKeyLatencyMs || 0) > 0)
    .sort((left, right) => (left.apiKeyLatencyMs || Number.MAX_SAFE_INTEGER) - (right.apiKeyLatencyMs || Number.MAX_SAFE_INTEGER))
    .slice(0, 5);
  const issueKeyAccounts = uniqueAccounts([...problemKeyAccounts, ...staleKeyAccounts, ...uncheckedKeyAccounts]).slice(0, 5);
  const successKeyAccounts = validKeyAccounts
    .slice()
    .sort((left, right) => (right.apiKeyModelUsable ? 1 : 0) - (left.apiKeyModelUsable ? 1 : 0) || (left.apiKeyLatencyMs || Number.MAX_SAFE_INTEGER) - (right.apiKeyLatencyMs || Number.MAX_SAFE_INTEGER))
    .slice(0, 3);
  const modelCoverage = buildModelCoverage(keyAccounts).slice(0, 7);
  const [message, setMessage] = useState("");
  const [showDetails, setShowDetails] = useState(false);
  const [keyTestBusyId, setKeyTestBusyId] = useState("");
  const [modelOverview, setModelOverview] = useState<ModelOverview | null>(null);
  const [pricingOverview, setPricingOverview] = useState<ModelPricingOverview | null>(null);
  const [modelSyncBusy, setModelSyncBusy] = useState(false);
  const [pricingSyncBusy, setPricingSyncBusy] = useState(false);
  const [keyExportPreview, setKeyExportPreview] = useState<KeyExportPreview | null>(null);
  const [keyExportBusy, setKeyExportBusy] = useState(false);
  const [cleanupPreview, setCleanupPreview] = useState<UnsupportedCheckinCleanupResult | null>(null);
  const [cleanupBusy, setCleanupBusy] = useState(false);
  const [cleanupIncludeLastUnsupported, setCleanupIncludeLastUnsupported] = useState(true);
  const keyTask = useTaskProgress();
  const balanceTask = useTaskProgress();

  // 批量任务完成后刷新数据
  useEffect(() => {
    if (keyTask.progress?.status === "done") {
      void onDone();
    }
  }, [keyTask.progress?.status, onDone]);

  useEffect(() => {
    if (balanceTask.progress?.status === "done") {
      void onDone();
    }
  }, [balanceTask.progress?.status, onDone]);
  const cleanupBatchLimit = cleanupPreview?.limit || UNSUPPORTED_CLEANUP_LIMIT;
  const cleanupCanDelete = Boolean(cleanupPreview?.matched && cleanupPreview.deleted === 0);
  const cleanupPreviewButtonLabel = cleanupBusy
    ? "处理中"
    : cleanupPreview?.deleted
      ? cleanupPreview.hasMore ? "继续预览下一批" : "再次检查"
      : cleanupPreview ? "重新预览" : "预览清理";
  const cleanupStatusLabel = cleanupPreview
    ? cleanupPreview.deleted
      ? cleanupPreview.hasMore ? "还有下一批" : "已清理"
      : cleanupPreview.matched
        ? cleanupPreview.hasMore ? "等待确认+" : "等待确认"
        : "无需清理"
    : "先预览";

  useEffect(() => {
    let cancelled = false;
    if (!keyAccounts.length) {
      setModelOverview(null);
      setKeyExportPreview(null);
      return;
    }
    void Promise.all([
      api<ModelOverview>("/api/models/overview"),
      api<ModelPricingOverview>("/api/models/pricing"),
    ])
      .then(([overview, pricing]) => {
        if (!cancelled) {
          setModelOverview(overview);
          setPricingOverview(pricing);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setModelOverview(null);
          setPricingOverview(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [keyAccounts.length, validKeyAccounts.length, usableModelAccounts.length, totalKnownModels]);

  async function testSingleKey(account: Account) {
    if (keyTestBusyId) return;
    setKeyTestBusyId(account.id);
    setMessage(`正在检测 ${account.displayName} 的 API Key…`);
    try {
      const result = await api<APIKeyTestResult>(`/api/accounts/${account.id}/test-api-key`, { method: "POST" });
      setMessage(`${account.displayName}：${formatAPIKeyTestMessage(result)}`);
      await onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "检测密钥失败");
    } finally {
      setKeyTestBusyId("");
    }
  }

  async function syncModels() {
    if (modelSyncBusy || !keyAccounts.length) return;
    setModelSyncBusy(true);
    setMessage("正在同步 Key 模型列表、可用性和测速…");
    try {
      const overview = await api<ModelOverview>("/api/models/sync", {
        method: "POST",
        body: JSON.stringify({ limit: 50 }),
      });
      setModelOverview(overview);
      const pricing = await api<ModelPricingOverview>("/api/models/pricing");
      setPricingOverview(pricing);
      setKeyExportPreview(null);
      setMessage(`模型同步完成：检测 ${overview.syncedAccounts || 0} 个 Key，覆盖 ${overview.modelCount} 个模型，价格来源 ${pricing.sourceCount} 条。`);
      await onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "模型同步失败");
    } finally {
      setModelSyncBusy(false);
    }
  }

  async function syncPricing() {
    if (pricingSyncBusy) return;
    setPricingSyncBusy(true);
    setMessage("正在探测 NewAPI/Sub2API 站点 /api/pricing，并写入本地缓存…");
    try {
      const pricing = await api<ModelPricingOverview>("/api/models/pricing/sync", {
        method: "POST",
        body: JSON.stringify({ limit: 50 }),
      });
      setPricingOverview(pricing);
      setMessage(`价格同步完成：在线缓存 ${pricing.liveCacheCount || 0} 个站点，价格来源 ${pricing.sourceCount} 条，对比模型 ${pricing.comparisons?.length || 0} 个。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "价格同步失败");
    } finally {
      setPricingSyncBusy(false);
    }
  }

  async function loadKeyExportPreview() {
    if (keyExportBusy || !keyAccounts.length) return;
    setKeyExportBusy(true);
    setMessage("正在生成 Key 安全导出预览…");
    try {
      const preview = await api<KeyExportPreview>("/api/keys/export-preview");
      setKeyExportPreview(preview);
      setMessage(`已生成脱敏导出预览：${preview.total} 个 Key，有效 ${preview.valid} 个，可用 ${preview.usable} 个。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "生成导出预览失败");
    } finally {
      setKeyExportBusy(false);
    }
  }

  async function copyKeyExportPreview() {
    const preview = keyExportPreview || (await api<KeyExportPreview>("/api/keys/export-preview"));
    setKeyExportPreview(preview);
    const body = JSON.stringify(preview, null, 2);
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(body);
      setMessage("已复制脱敏 Key 清单。真实 Key 不会被导出。");
      return;
    }
    downloadJSON("relaycheck-key-export-preview.json", body);
    setMessage("浏览器不支持剪贴板，已下载脱敏 Key 清单 JSON。");
  }

  function downloadKeyExportPreview() {
    if (!keyExportPreview) {
      void loadKeyExportPreview();
      return;
    }
    downloadJSON("relaycheck-key-export-preview.json", JSON.stringify(keyExportPreview, null, 2));
    setMessage("已下载脱敏 Key 清单 JSON。");
  }
  async function previewUnsupportedCheckinCleanup() {
    if (cleanupBusy) return;
    setCleanupBusy(true);
    setMessage("正在预览不支持签到账号，预览不会修改数据…");
    try {
      const result = await api<UnsupportedCheckinCleanupResult>("/api/accounts/delete-unsupported-checkins", {
        method: "POST",
        body: JSON.stringify({ limit: UNSUPPORTED_CLEANUP_LIMIT, dryRun: true, includeLastUnsupported: cleanupIncludeLastUnsupported }),
      });
      setCleanupPreview(result);
      setMessage(result.matched ? "预览到 " + result.matched + " 个不支持签到账号。本批最多处理 " + result.limit + " 个。" + (result.hasMore ? "删除后可继续预览下一批。" : "当前没有更多批次。") : "没有发现需要清理的不支持签到账号。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "预览不支持签到账号失败");
    } finally {
      setCleanupBusy(false);
    }
  }

  async function deleteUnsupportedCheckinCleanup() {
    if (cleanupBusy || !cleanupPreview?.matched) return;
    const samples = cleanupPreview.items.slice(0, 3).map((item) => item.upstreamSiteName + " / " + item.accountName).join("、");
    const confirmed = window.confirm("确认删除 " + cleanupPreview.matched + " 个不支持签到的账号？这会同步删除这些账号的签到日志和余额快照。" + (samples ? "\n样例：" + samples : ""));
    if (!confirmed) return;
    setCleanupBusy(true);
    setMessage("正在删除不支持签到账号…");
    try {
      const result = await api<UnsupportedCheckinCleanupResult>("/api/accounts/delete-unsupported-checkins", {
        method: "POST",
        body: JSON.stringify({ limit: UNSUPPORTED_CLEANUP_LIMIT, dryRun: false, includeLastUnsupported: cleanupIncludeLastUnsupported }),
      });
      setCleanupPreview(result);
      setMessage("已删除 " + result.deleted + " 个不支持签到账号。真实清理只通过 API 执行，未直接改数据库。" + (result.hasMore ? "仍有下一批，请继续预览后再删除。" : "可再次预览确认是否归零。"));
      await onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "删除不支持签到账号失败");
    } finally {
      setCleanupBusy(false);
    }
  }

  return (
    <div className="account-insight-strip">
      <div className="mini-stats">
        <span>成功 {successful.length}</span>
        <span>需授权 {pendingAuth.length}</span>
        <span>有密钥 {keyAccounts.length}</span>
        <span>本地疑似 {localSuspects.length}</span>
      </div>
      <div className="account-key-overview" aria-label="API Key 与模型能力总览">
        <div className="account-key-card">
          <span>有效 Key</span>
          <strong>{validKeyAccounts.length}/{keyAccounts.length}</strong>
          <em>异常 {problemKeyAccounts.length} · 未测 {uncheckedKeyAccounts.length}</em>
        </div>
        <div className="account-key-card">
          <span>模型能力</span>
          <strong>{totalKnownModels || "-"}</strong>
          <em>{usableModelAccounts.length} 个账号测速可用</em>
        </div>
        <div className={`account-key-card ${staleKeyAccounts.length ? "is-warning" : ""}`}>
          <span>建议重测</span>
          <strong>{staleKeyAccounts.length}</strong>
          <em>{averageLatency ? `平均 ${averageLatency}ms` : "还没有测速数据"}</em>
        </div>
        {modelSamples.length ? (
          <div className="account-key-card sample-card">
            <span>模型样例</span>
            <strong>{modelSamples.slice(0, 2).join(" / ")}</strong>
            <em>{modelSamples.slice(2).join(" / ") || "覆盖样例已足够"}</em>
          </div>
        ) : null}
      </div>
      <div className="account-capability-board" aria-label="模型测速排行与 Key 问题清单">
        <div className="account-capability-panel">
          <div className="capability-panel-head">
            <div>
              <span>模型测速排行</span>
              <strong>{speedRankAccounts.length ? `${speedRankAccounts[0].apiKeyLatencyMs}ms` : "-"}</strong>
            </div>
            <em>{speedRankAccounts.length ? `已测速 ${speedRankAccounts.length}` : "暂无测速"}</em>
          </div>
          <div className="capability-list">
            {speedRankAccounts.map((account) => (
              <div className="capability-row" key={`speed-${account.id}`}>
                <div>
                  <strong title={account.displayName}>{account.displayName}</strong>
                  <span title={`${account.upstreamSiteName} · ${account.apiKeyTestModel || "未记录模型"}`}>
                    {account.upstreamSiteName} · {account.apiKeyTestModel || "未记录模型"}
                  </span>
                </div>
                <b>{account.apiKeyLatencyMs}ms</b>
              </div>
            ))}
            {!speedRankAccounts.length ? <span className="capability-empty">{keyAccounts.length ? "批量检测密钥后会出现可用模型测速排行。" : "添加或导入 API Key 后，这里会显示模型测速排行。"}</span> : null}
          </div>
        </div>
        <div className="account-capability-panel is-actionable key-status-panel">
          <div className="capability-panel-head">
            <div>
              <span>Key 状态</span>
              <strong>{validKeyAccounts.length}/{keyAccounts.length}</strong>
            </div>
            <em>{issueKeyAccounts.length ? `${issueKeyAccounts.length} 个待处理` : keyAccounts.length ? "状态较新" : "未保存 Key"}</em>
          </div>
          <div className="capability-list">
            {successKeyAccounts.map((account) => (
              <div className="capability-row success-row" key={`success-${account.id}`}>
                <div>
                  <strong title={account.displayName}>{account.displayName}</strong>
                  <span title={`${account.upstreamSiteName} · ${account.apiKeyTestModel || "模型未测速"}`}>
                    {account.upstreamSiteName} · {account.apiKeyTestModel || "Key 有效"}
                  </span>
                </div>
                <b>{account.apiKeyLatencyMs ? `${account.apiKeyLatencyMs}ms` : "有效"}</b>
              </div>
            ))}
            {issueKeyAccounts.map((account) => (
              <div className="capability-row issue-row" key={`issue-${account.id}`}>
                <div>
                  <strong title={account.displayName}>{account.displayName}</strong>
                  <span title={`${account.upstreamSiteName} · ${keyIssueLabel(account)}`}>{account.upstreamSiteName} · {keyIssueLabel(account)}</span>
                </div>
                <button type="button" className="ghost" disabled={keyTestBusyId !== ""} onClick={() => void testSingleKey(account)}>
                  {keyTestBusyId === account.id ? "检测中" : "检测"}
                </button>
              </div>
            ))}
            {!successKeyAccounts.length && !issueKeyAccounts.length ? <span className="capability-empty">{keyAccounts.length ? "已保存 Key 暂无明显异常。" : "在账号卡编辑里保存 API Key，或从配置/密码文件导入后即可检测。"}</span> : null}
          </div>
        </div>
        <div className="account-capability-panel model-coverage-panel">
          <div className="capability-panel-head">
            <div>
              <span>模型覆盖</span>
              <strong>{modelOverview?.modelCount || modelCoverage.length || "-"}</strong>
            </div>
            <em>{modelOverview?.fastestLatencyMs ? `最快 ${modelOverview.fastestLatencyMs}ms` : modelSamples.length ? "可筛选账号" : "暂无样例"}</em>
          </div>
          <div className="model-coverage-list">
            {(modelOverview?.models?.length ? modelOverview.models.slice(0, 9).map((item) => ({
              model: item.model,
              accountCount: item.accountCount,
              siteSamples: item.sites || [],
            })) : modelCoverage).map((item) => (
              <button
                type="button"
                className="model-coverage-chip"
                key={item.model}
                onClick={() => onModelFilter?.(item.model)}
                title={`${item.model} · ${item.accountCount} 个账号 · ${item.siteSamples.join("、")}`}
              >
                <span>{item.model}</span>
                <b>{item.accountCount}</b>
              </button>
            ))}
            {!(modelOverview?.models?.length || modelCoverage.length) ? <span className="capability-empty">检测 API Key 后，这里会按模型聚合可用账号。</span> : null}
          </div>
        </div>
        <div className="account-capability-panel price-compare-panel">
          <div className="capability-panel-head">
            <div>
              <span>模型价格雷达</span>
              <strong>{pricingOverview?.comparisons?.length || pricingOverview?.sourceCount || modelOverview?.priceHints?.length || "-"}</strong>
            </div>
            <em>{pricingOverview?.sourceCount ? `${pricingOverview.liveCacheCount || 0} 在线缓存 · ${pricingOverview.ratioCount} 倍率` : modelOverview ? "轻量分层" : "待同步"}</em>
          </div>
          <div className="capability-list">
            {(pricingOverview?.comparisons || []).slice(0, 4).map((item) => (
              <div className="capability-row price-row comparison-row" key={`compare-${item.model}`}>
                <div>
                  <strong title={`${item.model} · ${item.notes || ""}`}>{item.model}</strong>
                  <span>{formatPriceComparisonMeta(item)}</span>
                </div>
                <b>{formatPriceComparisonBadge(item)}</b>
              </div>
            ))}
            {(pricingOverview?.siteCaches || []).slice(0, 2).map((item) => (
              <div className={`capability-row cache-row ${item.status === "success" ? "success-row" : "issue-row"}`} key={`pricing-cache-${item.siteId}`}>
                <div>
                  <strong title={item.baseUrl}>{item.siteName}</strong>
                  <span>{pricingCacheStatusLabel(item.status)} · {item.modelCount} 模型 · {item.latencyMs ? `${item.latencyMs}ms` : "未测速"}</span>
                </div>
                <b>{item.httpStatus || item.status}</b>
              </div>
            ))}
            {(pricingOverview?.sources || []).slice(0, 5).map((source) => (
              <div className="capability-row price-row" key={`pricing-${source.channelId}-${source.model}-${source.fieldPath}`}>
                <div>
                  <strong title={`${source.model} · ${source.fieldPath}`}>{source.model}</strong>
                  <span>{source.channelName} · {formatPricingSource(source)}</span>
                </div>
                <b>{pricingSourceBadge(source)}</b>
              </div>
            ))}
            {!pricingOverview?.sources?.length && (modelOverview?.priceHints || []).slice(0, 5).map((hint) => (
              <div className="capability-row price-row" key={`price-${hint.model}`}>
                <div>
                  <strong title={hint.model}>{hint.model}</strong>
                  <span>{hint.vendor} · {priceLevelLabel(hint.priceLevel)}</span>
                </div>
                <b>{priceLevelShort(hint.priceLevel)}</b>
              </div>
            ))}
            {!pricingOverview?.sources?.length && !modelOverview?.priceHints?.length ? <span className="capability-empty">先同步模型或同步 NewAPI 渠道；需要更完整价格时点"同步在线价格"。检测仍在本地直连上游完成，不向第三方提交 Key。</span> : null}
          </div>
          <div className="mini-action-row">
            <button type="button" className="ghost" disabled={pricingSyncBusy} onClick={() => void syncPricing()}>
              {pricingSyncBusy ? "探测中" : "同步在线价格"}
            </button>
            <span className="capability-mini-note">{pricingOverview?.failedCacheCount ? `${pricingOverview.failedCacheCount} 个站点未返回价格` : "参考 modeloc 类检测维度，本地执行"}</span>
          </div>
        </div>
        <div className="account-capability-panel key-export-panel is-actionable">
          <div className="capability-panel-head">
            <div>
              <span>Key 安全导出</span>
              <strong>{keyExportPreview?.total ?? keyAccounts.length}</strong>
            </div>
            <em>{keyExportPreview ? `有效 ${keyExportPreview.valid}` : "仅脱敏"}</em>
          </div>
          <div className="capability-list">
            {(keyExportPreview?.items || []).slice(0, 3).map((item) => (
              <div className={`capability-row ${item.modelUsable ? "success-row" : "issue-row"}`} key={`export-${item.accountId}`}>
                <div>
                  <strong title={item.maskedExportRef}>{item.accountName}</strong>
                  <span>{item.siteName} · {item.fingerprint} · {apiKeyStatusLabel(item.status)}</span>
                </div>
                <b>{item.latencyMs ? `${item.latencyMs}ms` : item.modelUsable ? "可用" : "待测"}</b>
              </div>
            ))}
            {!keyExportPreview ? <span className="capability-empty">导出前先生成预览。导出内容只包含 Key 指纹、状态、模型和测速，不包含真实密钥。</span> : null}
          </div>
          <div className="mini-action-row">
            <button type="button" className="ghost" disabled={!keyAccounts.length || keyExportBusy} onClick={() => void loadKeyExportPreview()}>{keyExportBusy ? "生成中" : "预览"}</button>
            <button type="button" className="ghost" disabled={!keyAccounts.length} onClick={() => void copyKeyExportPreview()}>复制脱敏</button>
            <button type="button" className="ghost" disabled={!keyExportPreview} onClick={downloadKeyExportPreview}>下载</button>
          </div>
        </div>
        <div className="account-capability-panel unsupported-cleanup-panel is-actionable">
          <div className="capability-panel-head">
            <div>
              <span>签到清理</span>
              <strong>{cleanupPreview?.matched ?? unsupportedCheckinAccounts.length}</strong>
            </div>
            <em>{cleanupStatusLabel}</em>
          </div>
          <label className="cleanup-option">
            <input
              type="checkbox"
              checked={cleanupIncludeLastUnsupported}
              onChange={(event) => {
                setCleanupIncludeLastUnsupported(event.currentTarget.checked);
                setCleanupPreview(null);
              }}
            />
            包含上次签到返回“不支持”的账号
          </label>
          <div className="capability-list cleanup-preview-list">
            {(cleanupPreview?.items || []).slice(0, 5).map((item) => (
              <div className="capability-row issue-row" key={"cleanup-" + item.accountId}>
                <div>
                  <strong title={item.accountName}>{item.accountName}</strong>
                  <span title={item.upstreamSiteName + " · " + item.upstreamSiteKind}>{item.upstreamSiteName} · {cleanupReasonLabel(item.reason)}</span>
                </div>
                <b>{item.lastCheckinStatus || "site"}</b>
              </div>
            ))}
            {cleanupPreview && cleanupPreview.items.length > 5 ? <span className="capability-empty">还有 {cleanupPreview.items.length - 5} 个账号未展开；本批接口最多处理 {cleanupBatchLimit} 个。</span> : null}
            {cleanupPreview?.hasMore ? <span className="capability-empty">后面还有下一批候选账号；当前批次上限 {cleanupBatchLimit} 个，删除后请继续预览。</span> : null}
            {cleanupPreview && cleanupPreview.deleted > 0 ? <span className="capability-empty">本批已通过 API 删除 {cleanupPreview.deleted} 个账号；再次预览会读取下一批或确认已归零。</span> : null}
            {!cleanupPreview ? <span className="capability-empty">先预览将要删除的账号；预览模式不会写入数据库。</span> : null}
            {cleanupPreview && !cleanupPreview.items.length ? <span className="capability-empty">当前没有匹配的不支持签到账号。</span> : null}
          </div>
          <div className="mini-action-row">
            <button type="button" className="ghost" disabled={cleanupBusy} onClick={() => void previewUnsupportedCheckinCleanup()}>
              {cleanupPreviewButtonLabel}
            </button>
            <button type="button" className="danger" disabled={cleanupBusy || !cleanupCanDelete} onClick={() => void deleteUnsupportedCheckinCleanup()}>
              删除本批
            </button>
          </div>
        </div>
      </div>
      <div className="toolbar">
        <button
          type="button"
          disabled={!keyAccounts.length || keyTask.loading || keyTask.progress?.status === "running"}
          onClick={() => void keyTask.startTask("test_keys")}
        >
          {keyTask.loading || keyTask.progress?.status === "running" ? "测试中…" : "批量测试 Key"}
        </button>
        <button
          type="button"
          disabled={balanceTask.loading || balanceTask.progress?.status === "running"}
          onClick={() => void balanceTask.startTask("refresh_balances")}
        >
          {balanceTask.loading || balanceTask.progress?.status === "running" ? "刷新中…" : "批量刷新余额"}
        </button>
        <button
          onClick={async () => {
            setMessage("正在用已保存密码重登…");
            const result = await api<BulkPasswordLoginResponse>("/api/accounts/bulk-password-login", {
              method: "POST",
              body: JSON.stringify({ limit: 20 }),
            });
            setMessage(`密码重登处理 ${result.processed} 个，成功 ${result.success} 个，失败 ${result.failed} 个。`);
            await onDone();
          }}
        >
          批量密码重登
        </button>
        <button
          disabled={!keyAccounts.length}
          onClick={() => void syncModels()}
        >
          {modelSyncBusy ? "同步中…" : "同步模型/密钥"}
        </button>
        <button
          onClick={async () => {
            setMessage("正在批量打开网页登录窗口…");
            const result = await api<BulkBrowserOpenResponse>("/api/accounts/bulk-open-browser-login", {
              method: "POST",
              body: JSON.stringify({ limit: 5 }),
            });
            setMessage(`网页登录已打开/复用 ${result.opened} 个，失败 ${result.failed} 个。登录完成后点"批量保存已登录"。`);
          }}
        >
          批量打开授权
        </button>
        <button
          onClick={async () => {
            setMessage("正在保存已完成网页登录的账号…");
            const result = await api<BulkBrowserSaveResponse>("/api/accounts/bulk-finish-browser-login", {
              method: "POST",
              body: JSON.stringify({}),
            });
            setMessage(`保存授权 ${result.saved} 个，失败/未完成 ${result.failed} 个。`);
            await onDone();
          }}
        >
          批量保存已登录
        </button>
        <button type="button" className="ghost" onClick={() => setShowDetails((current) => !current)}>{showDetails ? "收起明细" : "展开明细"}</button>
      </div>
      {keyTask.progress || keyTask.loading || keyTask.error ? (
        <TaskProgressView
          progress={keyTask.progress}
          loading={keyTask.loading}
          error={keyTask.error}
          onCancel={keyTask.cancelTask}
          onDismiss={keyTask.reset}
          labels={LABELS_TEST_KEYS}
        />
      ) : null}
      {balanceTask.progress || balanceTask.loading || balanceTask.error ? (
        <TaskProgressView
          progress={balanceTask.progress}
          loading={balanceTask.loading}
          error={balanceTask.error}
          onCancel={balanceTask.cancelTask}
          onDismiss={balanceTask.reset}
          labels={LABELS_REFRESH_BALANCE}
        />
      ) : null}
      {message ? <span className="muted">{message}</span> : null}
      {showDetails ? (
        <div className="account-insight-details">
          <div>
            <strong>待网页登录授权</strong>
            {pendingAuth.slice(0, 5).map((account) => (
              <div className="compact-row" key={account.id}>
                <span>{account.upstreamSiteName} · {account.email || account.username || account.displayName}</span>
                <button
                  onClick={async () => {
                    await api(`/api/accounts/${account.id}/open-browser-login`, { method: "POST" });
                  }}
                >
                  打开
                </button>
              </div>
            ))}
            {!pendingAuth.length ? <EmptyState title="暂无待授权账号" description="当前账号授权状态看起来不错。" /> : null}
          </div>
          <div>
            <strong>本地地址疑似误匹配</strong>
            {localSuspects.slice(0, 5).map((account) => (
              <div className="compact-row" key={account.id}>
                <span>{account.upstreamSiteBaseUrl} · {account.email || account.username || account.displayName}</span>
                <button
                  className="danger"
                  onClick={async () => {
                    const confirmed = window.confirm(`确认删除疑似误匹配账号"${account.displayName}"？这会删除该账号保存的本地凭据。`);
                    if (!confirmed) return;
                    await api(`/api/accounts/${account.id}`, { method: "DELETE" });
                    await onDone();
                  }}
                >
                  删除
                </button>
              </div>
            ))}
            {!localSuspects.length ? <EmptyState title="没有本地误匹配" description="未发现 localhost 或 127.0.0.1 误绑定到远程账号。" /> : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
