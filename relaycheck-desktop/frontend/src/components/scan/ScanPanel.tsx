import { memo, useState } from "react";

import { localNewapiApi, type AutoDetectResponse } from "@/api/local-newapi";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LineIcon } from "@/components/ui/line-icon";
import { LocalNewAPISyncPanel } from "@/components/scan/LocalNewAPISyncPanel";
import type { NavigationIntent, TabKey } from "@/types";
import "@/styles/domains/scan.css";

type ScanPanelProps = {
  onRefresh: () => Promise<void>;
  onNavigate: (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;
};

/** 仅当至少一条结果有实际导入/建站/合并时，才展示扫描成功后的导航动作。 */
export function hasSuccessfulScanImport(result: AutoDetectResponse) {
  return (
    result.found &&
    result.results.some(
      (item) => !item.error && (item.importedCount > 0 || item.sitesCreated > 0 || item.sitesMerged > 0),
    )
  );
}

/** 错误结果用 assertive alert，成功/中性用 polite status。 */
export function scanFeedbackProps(hasErrors: boolean) {
  return hasErrors
    ? ({ role: "alert", "aria-live": "assertive", "aria-atomic": true } as const)
    : ({ role: "status", "aria-live": "polite", "aria-atomic": true } as const);
}

function ScanPanelBase({ onRefresh, onNavigate }: ScanPanelProps) {
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<AutoDetectResponse | null>(null);

  /** 触发本机 SQLite 自动探测导入；失败写入稳定文案，不抛未处理拒绝。 */
  async function handleScan() {
    setBusy(true);
    setResult(null);
    try {
      const data = await localNewapiApi.autoDetectImport();
      setResult(data);
      if (data.found) {
        await onRefresh();
      }
    } catch {
      setResult({
        found: false,
        message: "扫描请求失败，请检查服务状态。",
        results: [],
      });
    } finally {
      setBusy(false);
    }
  }

  const hasErrors = Boolean(result?.results.some((r) => r.error) || result?.message.includes("失败"));
  const hasSuccessfulImport = result ? hasSuccessfulScanImport(result) : false;

  return (
    <section className="scan-panel" aria-label="本机 NewAPI 扫描" aria-busy={busy}>
      <Card>
        <CardHeader>
          <CardTitle>本机 NewAPI 扫描</CardTitle>
        </CardHeader>
        <CardContent className="scan-card-content">
          <p className="scan-copy text-sm text-muted-foreground">
            自动检测本机常见位置（如 <code>D:\newapi\data\one-api.db</code>）的 NewAPI SQLite 数据库，
            识别其中的渠道数据并导入到 RelayCheck。
          </p>
          <div>
            <Button
              onClick={handleScan}
              disabled={busy}
              size="lg"
              aria-busy={busy}
              aria-label={busy ? "正在扫描本机 NewAPI 数据库" : "检测并导入本机 NewAPI 数据库"}
            >
              {busy ? (
                <>
                  <span className="spinner" aria-hidden="true" />
                  扫描中…
                </>
              ) : (
                <>
                  <LineIcon name="scan" />
                  检测并导入
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      {result ? (
        <Card {...scanFeedbackProps(hasErrors)}>
          <CardHeader>
            <CardTitle>
              <span className="scan-result-title">
                {result.found ? <LineIcon name="success" /> : <LineIcon name="info" />}
                扫描结果
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="scan-card-content">
            <p className="scan-copy text-sm">{result.message}</p>

            {result.results.length > 0 ? (
              <div className="scan-result-list">
                {result.results.map((item) => (
                  <div key={item.dbPath} className={`scan-result-row ${item.error ? "has-error" : ""}`}>
                    <code className="scan-result-path text-xs">{item.dbPath}</code>
                    {item.error ? (
                      <Badge variant="destructive">{item.error}</Badge>
                    ) : (
                      <>
                        <Badge variant="success">{item.importedCount} 条渠道</Badge>
                        {item.sitesCreated > 0 ? <Badge variant="default">+{item.sitesCreated} 站点</Badge> : null}
                        {item.sitesMerged > 0 ? <Badge variant="secondary">{item.sitesMerged} 合并</Badge> : null}
                        <code className="text-xs text-muted-foreground">{item.baseUrl}</code>
                      </>
                    )}
                  </div>
                ))}
              </div>
            ) : null}

            {hasErrors ? (
              <p className="scan-copy text-xs text-muted-foreground">
                部分数据库导入失败，可检查数据库文件是否完整或权限是否正确。
              </p>
            ) : null}

            {hasSuccessfulImport ? (
              <div className="scan-next-actions" aria-label="扫描结果下一步">
                <Button variant="secondary" type="button" onClick={() => onNavigate("channels")}>
                  <LineIcon name="channels" />
                  查看渠道
                </Button>
                <Button type="button" onClick={() => onNavigate("sites", { accountsView: "all" })}>
                  <LineIcon name="accounts" />
                  前往站点与账号
                </Button>
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {!busy && !result ? (
        <Card {...scanFeedbackProps(false)}>
          <CardContent>
            <p className="scan-empty text-sm text-muted-foreground">点击上方按钮开始扫描本机 NewAPI 数据库</p>
          </CardContent>
        </Card>
      ) : null}

      <LocalNewAPISyncPanel onRefresh={onRefresh} />
    </section>
  );
}

export const ScanPanel = memo(ScanPanelBase);
