import { memo, useCallback, useEffect, useState } from "react";

import { api } from "@/api/client";
import {
  formatImportCountersMessage,
  instanceNeedsCredential,
  syncCapabilityLabel,
  syncTokenStatusLabel,
  type ImportCounters,
} from "@/lib/syncFeedback";
import type { LocalNewAPIInstance } from "@/types";

export type LocalNewAPISyncPanelProps = {
  onRefresh: () => Promise<void>;
};

type SyncResultMessage = {
  instanceId: string;
  level: "success" | "warning" | "danger" | "info";
  text: string;
};

function LocalNewAPISyncPanelBase({ onRefresh }: LocalNewAPISyncPanelProps) {
  const [instances, setInstances] = useState<LocalNewAPIInstance[]>([]);
  const [loading, setLoading] = useState(false);
  const [busyId, setBusyId] = useState("");
  const [tokenDrafts, setTokenDrafts] = useState<Record<string, string>>({});
  const [messages, setMessages] = useState<Record<string, SyncResultMessage>>({});
  const [listError, setListError] = useState("");

  const loadInstances = useCallback(async () => {
    setLoading(true);
    setListError("");
    try {
      const list = await api<LocalNewAPIInstance[]>("/api/local-newapi");
      setInstances(Array.isArray(list) ? list : []);
    } catch (error) {
      setListError(error instanceof Error ? error.message : "加载 NewAPI 实例失败");
      setInstances([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadInstances();
  }, [loadInstances]);

  function setMessage(instanceId: string, level: SyncResultMessage["level"], text: string) {
    setMessages((current) => ({
      ...current,
      [instanceId]: { instanceId, level, text },
    }));
  }

  async function runSync(instance: LocalNewAPIInstance) {
    setBusyId(instance.id);
    try {
      const body: Record<string, unknown> = {
        importKeys: false,
        detectAfterImport: false,
        pageSize: 100,
      };
      const draft = (tokenDrafts[instance.id] || "").trim();
      if (draft) {
        body.accessToken = draft;
        body.saveAccessToken = true;
      }
      const result = await api<ImportCounters & { instanceId?: string }>(
        `/api/local-newapi/${instance.id}/sync`,
        {
          method: "POST",
          body: JSON.stringify(body),
        },
      );
      const text = formatImportCountersMessage(result);
      const level =
        (result.importedCount ?? 0) > 0
          ? "success"
          : (result.skippedExcluded ?? 0) > 0
            ? "warning"
            : (result.fetchedCount ?? 0) === 0
              ? "info"
              : "warning";
      setMessage(instance.id, level, text);
      if (draft) {
        setTokenDrafts((current) => ({ ...current, [instance.id]: "" }));
      }
      await loadInstances();
      await onRefresh();
    } catch (error) {
      setMessage(
        instance.id,
        "danger",
        formatImportCountersMessage(
          {},
          {
            error: error instanceof Error ? error.message : "同步失败",
          },
        ),
      );
    } finally {
      setBusyId("");
    }
  }

  return (
    <section className="local-newapi-sync-panel" aria-label="NewAPI 实例同步">
      <div className="card local-newapi-sync-head">
        <div>
          <strong>NewAPI 实例同步</strong>
          <p>
            展示同步能力、访问令牌状态与结果计数。渠道同步使用系统访问令牌或本机数据库路径，
            与账号网页登录 / 2FA 无关。
          </p>
        </div>
        <button
          type="button"
          className="ghost"
          disabled={loading || Boolean(busyId)}
          onClick={() => void loadInstances()}
        >
          {loading ? "刷新中…" : "刷新实例"}
        </button>
      </div>

      {listError ? <div className="error">{listError}</div> : null}

      <div className="local-newapi-instance-list">
        {instances.map((instance) => {
          const needsCred = instanceNeedsCredential(instance);
          const msg = messages[instance.id];
          const busy = busyId === instance.id;
          return (
            <article
              key={instance.id}
              className={`card local-newapi-instance-card ${needsCred ? "needs-credential" : ""}`}
            >
              <div className="local-newapi-instance-head">
                <div>
                  <span>{instance.status || "unknown"}</span>
                  <strong title={instance.name}>{instance.name}</strong>
                  <em title={instance.baseUrl}>{instance.baseUrl || "-"}</em>
                </div>
                <div className="local-newapi-chips">
                  <span className="chip">{syncCapabilityLabel(instance.syncCapability)}</span>
                  <span className={`chip ${instance.hasSyncToken ? "ok" : "warn"}`}>
                    {instance.hasSyncToken
                      ? `令牌 ${instance.syncTokenMasked || "已保存"}`
                      : "未保存令牌"}
                  </span>
                  <span className="chip">渠道 {instance.channelCount ?? 0}</span>
                </div>
              </div>

              <p className="local-newapi-hint">
                {syncTokenStatusLabel(instance.hasSyncToken, instance.syncCapability)}
              </p>

              {instance.databasePath ? (
                <p className="local-newapi-db">数据库路径已配置（本地同步）</p>
              ) : null}

              <div className="local-newapi-token-row">
                <label className="field">
                  <span>系统访问令牌（可选，同步时保存）</span>
                  <input
                    type="password"
                    value={tokenDrafts[instance.id] || ""}
                    onChange={(event) =>
                      setTokenDrafts((current) => ({ ...current, [instance.id]: event.target.value }))
                    }
                    placeholder="NewAPI 后台 → 个人设置 → 访问令牌"
                    autoComplete="new-password"
                    disabled={busy}
                  />
                </label>
              </div>

              <div className="toolbar">
                <button type="button" disabled={busy || loading} onClick={() => void runSync(instance)}>
                  {busy ? "同步中…" : "同步渠道"}
                </button>
              </div>

              {msg ? (
                <div
                  className={
                    msg.level === "danger" ? "error" : msg.level === "warning" ? "problem-hint" : "note"
                  }
                  role={msg.level === "danger" ? "alert" : "status"}
                >
                  {msg.text}
                </div>
              ) : null}
            </article>
          );
        })}

        {!loading && !instances.length ? (
          <div className="empty-state">
            <div className="empty-mark">NA</div>
            <strong>暂无 NewAPI 实例</strong>
            <span>
              可先使用上方「检测并导入」扫描本机数据库，或在引导中填写后台地址与访问令牌。
            </span>
          </div>
        ) : null}
      </div>
    </section>
  );
}

export const LocalNewAPISyncPanel = memo(LocalNewAPISyncPanelBase);