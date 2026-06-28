import { useState } from "react";

import { api } from "@/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LineIcon } from "@/components/ui/line-icon";

type AutoDetectResultItem = {
  dbPath: string;
  baseUrl: string;
  importedCount: number;
  sitesCreated: number;
  sitesMerged: number;
  error?: string;
};

type AutoDetectResponse = {
  found: boolean;
  message: string;
  results: AutoDetectResultItem[];
};

type ScanPanelProps = {
  onRefresh: () => Promise<void>;
};

export function ScanPanel({ onRefresh }: ScanPanelProps) {
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<AutoDetectResponse | null>(null);

  async function handleScan() {
    setBusy(true);
    setResult(null);
    try {
      const data = await api<AutoDetectResponse>("/api/local-newapi/auto-detect-import", { method: "POST" });
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

  const hasErrors = result?.results.some((r) => r.error);

  return (
    <section className="scan-panel" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <Card>
        <CardHeader>
          <CardTitle>本机 NewAPI 扫描</CardTitle>
        </CardHeader>
        <CardContent style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <p className="text-sm text-muted-foreground" style={{ margin: 0, lineHeight: 1.6 }}>
            自动检测本机常见位置（如 <code>D:\newapi\data\one-api.db</code>）的 NewAPI SQLite 数据库，
            识别其中的渠道数据并导入到 RelayCheck。
          </p>
          <div>
            <Button onClick={handleScan} disabled={busy} size="lg">
              {busy ? (
                <>
                  <span className="spinner" style={{ display: "inline-block", width: 14, height: 14, border: "2px solid currentColor", borderTopColor: "transparent", borderRadius: "50%", animation: "spin 0.6s linear infinite" }} />
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
        <Card>
          <CardHeader>
            <CardTitle>
              <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
                {result.found ? (
                  <LineIcon name="success" />
                ) : (
                  <LineIcon name="info" />
                )}
                扫描结果
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <p className="text-sm" style={{ margin: 0 }}>{result.message}</p>

            {result.results.length > 0 ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {result.results.map((item, i) => (
                  <div
                    key={i}
                    style={{
                      display: "flex",
                      flexWrap: "wrap",
                      alignItems: "center",
                      gap: 8,
                      padding: "10px 12px",
                      borderRadius: 10,
                      border: "1px solid var(--border, #e2e8f0)",
                      background: item.error ? "rgba(239,68,68,0.04)" : undefined,
                    }}
                  >
                    <code className="text-xs" style={{ flex: "1 1 240px", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {item.dbPath}
                    </code>
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
              <p className="text-xs text-muted-foreground" style={{ margin: 0 }}>
                部分数据库导入失败，可检查数据库文件是否完整或权限是否正确。
              </p>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {!busy && !result ? (
        <Card>
          <CardContent>
            <p className="text-sm text-muted-foreground" style={{ margin: 0, textAlign: "center", padding: "24px 0" }}>
              点击上方按钮开始扫描本机 NewAPI 数据库
            </p>
          </CardContent>
        </Card>
      ) : null}
    </section>
  );
}
