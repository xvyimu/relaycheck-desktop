import { reopenOnboarding } from "@/components/onboarding/OnboardingWizard";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusLabel } from "@/components/ui/status-label";
import { formatBuildTime, formatTime } from "@/lib/format";
import { auditActionLabel, auditLevelLabel, diagnosticLevelLabel, schedulerStatusLabel } from "@/lib/labels";
import { safeExternalUrl } from "@/lib/safeExternalUrl";
import type { AuditLogItem, ChannelHealthScheduleConfig, PortCheckResult, SchedulerJobStatus, SchedulerStatus, StatusPayload, SyncScheduleConfig, SystemSetting, VersionCheckResult } from "@/types";

export function SettingsAboutCard({
  status,
  scheduler,
  checkinJob,
}: {
  status: StatusPayload;
  scheduler: SchedulerStatus | null;
  checkinJob?: SchedulerJobStatus;
}) {
  return (
    <article className="card settings-about-card">
      <div className="section-heading">
        <div>
          <strong>关于 / 版本</strong>
          <span>{status.productName} &middot; {status.productVersion}</span>
        </div>
        <span className="status-pill success"><StatusLabel level="success" label="正式版" /></span>
      </div>
      <div className="detail-list">
        <div><span>显示名</span><strong>{status.productName}</strong></div>
        <div><span>版本</span><strong>{status.productVersion}</strong></div>
        <div><span>构建时间</span><strong>{formatBuildTime(status.buildTime)}</strong></div>
        <div><span>绑定地址</span><strong>{status.bindAddress}:{status.port}</strong></div>
        {status.portConflict && status.preferredPort ? (
          <div className="warning-banner" style={{ marginTop: 8, padding: "8px 12px", borderRadius: 8, fontSize: 13 }}>
            <span>端口冲突</span>
            <strong>首选端口 {status.preferredPort} 被占用，已回退到 {status.port}</strong>
          </div>
        ) : null}
        <div><span>调度器</span><strong>{scheduler ? `${scheduler.jobs.length} 个任务 &middot; ${schedulerStatusLabel(checkinJob?.status || "idle")}` : "读取中"}</strong></div>
        <div>
          <span>上次自检</span>
          <strong>{status.lastDiagnostics ? `${diagnosticLevelLabel(status.lastDiagnostics.overall)} &middot; ${status.lastDiagnostics.itemCount} 项 &middot; ${formatTime(status.lastDiagnostics.generatedAt)}` : "未生成"}</strong>
        </div>
      </div>
    </article>
  );
}

export function SettingsVersionCheckCard({
  status,
  versionCheckURL,
  currentVersionCheckURL,
  versionChecking,
  versionCheckResult,
  onURLChange,
  onPersistURL,
  onCheck,
}: {
  status: StatusPayload;
  versionCheckURL: string;
  currentVersionCheckURL: string;
  versionChecking: boolean;
  versionCheckResult: VersionCheckResult | null;
  onURLChange: (value: string) => void;
  onPersistURL: () => void;
  onCheck: () => void;
}) {
  return (
    <article className="card settings-version-check-card">
      <div className="section-heading">
        <div>
          <strong>版本检查</strong>
          <span>检查是否有新版本可用</span>
        </div>
      </div>
      <div className="proxy-form-grid">
        <label className="field">
          <span>版本清单 URL</span>
          <input
            value={versionCheckURL}
            onChange={(event) => onURLChange(event.target.value)}
            onBlur={() => {
              if (versionCheckURL !== currentVersionCheckURL) onPersistURL();
            }}
            placeholder="https://example.com/relaycheck-version.json"
          />
        </label>
        <button type="button" disabled={versionChecking} onClick={onCheck}>
          {versionChecking ? "检查中…" : "检查更新"}
        </button>
      </div>
      {versionCheckResult ? (
        <div className="detail-list" style={{ marginTop: 8 }}>
          <div><span>当前版本</span><strong>{versionCheckResult.currentVersion || status.productVersion}</strong></div>
          {versionCheckResult.latestVersion ? <div><span>最新版本</span><strong>{versionCheckResult.latestVersion}</strong></div> : null}
          <div>
            <span>状态</span>
            <strong>
              {versionCheckResult.error
                ? versionCheckResult.error
                : versionCheckResult.updateAvailable
                  ? "有新版本可用"
                  : "已是最新版本"}
            </strong>
          </div>
          {versionCheckResult.updateAvailable && versionCheckResult.releaseUrl ? (
            <div>
              <span>下载</span>
              <strong>
                <a href={safeExternalUrl(versionCheckResult.releaseUrl) || undefined} target="_blank" rel="noopener noreferrer" style={{ color: "var(--v4-blue)" }}>
                  打开下载页面
                </a>
              </strong>
            </div>
          ) : null}
          {versionCheckResult.releaseNotes ? (
            <div style={{ marginTop: 4, padding: "8px 12px", background: "var(--v4-neutral-bg)", borderRadius: 8, fontSize: 13, whiteSpace: "pre-wrap" }}>
              {versionCheckResult.releaseNotes}
            </div>
          ) : null}
        </div>
      ) : null}
      <div className="problem-hint detail-hint">
        配置版本清单 URL 后，可检查远程是否有新版本。清单格式: {"{ \"version\": \"v1.1\", \"releaseUrl\": \"...\", \"releaseNotes\": \"...\" }"}
      </div>
    </article>
  );
}

export function SettingsPortCheckCard({
  status,
  portCheckPort,
  portChecking,
  portCheckResult,
  onPortChange,
  onCheck,
}: {
  status: StatusPayload;
  portCheckPort: string;
  portChecking: boolean;
  portCheckResult: PortCheckResult | null;
  onPortChange: (value: string) => void;
  onCheck: () => void;
}) {
  return (
    <article className="card settings-port-check-card">
      <div className="section-heading">
        <div>
          <strong>端口检测</strong>
          <span>检查本地端口是否可绑定</span>
        </div>
      </div>
      <div className="proxy-form-grid">
        <label className="field">
          <span>端口号</span>
          <input value={portCheckPort} onChange={(event) => onPortChange(event.target.value)} placeholder="如 3001" />
        </label>
        <button type="button" disabled={portChecking} onClick={onCheck}>
          {portChecking ? "检测中…" : "检测端口"}
        </button>
      </div>
      {portCheckResult ? (
        <div className="detail-list">
          <div><span>端口</span><strong>{portCheckResult.port}</strong></div>
          <div>
            <span>状态</span>
            <strong>{portCheckResult.available ? "可用（未被占用）" : portCheckResult.inUse ? "已被占用" : "检测失败"}</strong>
          </div>
          {portCheckResult.error ? <div><span>详情</span><strong>{portCheckResult.error}</strong></div> : null}
        </div>
      ) : null}
      <div className="problem-hint detail-hint">启动前检测端口可避免端口冲突。当前运行端口为 {status.port}。</div>
    </article>
  );
}

export function SettingsPathCard({ status }: { status: StatusPayload }) {
  return (
    <article className="card settings-path-card">
      <strong>本地路径</strong>
      <div className="detail-list">
        <div><span>数据库</span><strong>{status.databasePath}</strong></div>
        <div><span>备份目录</span><strong>{status.backupDir}</strong></div>
        <div><span>架构</span><strong>{status.architecture}</strong></div>
        <div><span>代理</span><strong>{status.networkProxy?.enabled ? status.networkProxy.urlMasked : "未启用"}</strong></div>
      </div>
      <div className="problem-hint detail-hint">建议在大量导入、批量识别、批量签到前先点一次"立即备份数据库"。</div>
    </article>
  );
}

export function SettingsHelpCard({ showHelpGuide, onToggle }: { showHelpGuide: boolean; onToggle: () => void }) {
  return (
    <article className="card settings-help-card">
      <div className="section-heading">
        <div>
          <strong>帮助 / 文档</strong>
          <span>把常用说明集中在本地设置页，避免需要翻目录才知道下一步。</span>
        </div>
        <div className="toolbar compact-toolbar">
          <Button variant="ghost" type="button" onClick={onToggle}>{showHelpGuide ? "收起" : "查看指引"}</Button>
          <Button variant="ghost" type="button" onClick={reopenOnboarding}>重新查看引导</Button>
        </div>
      </div>
      <div className="detail-list">
        <div><span>使用说明</span><strong>relaycheck-desktop/README.md</strong></div>
        <div><span>总清单</span><strong>relaycheck-desktop/PROMPT_CHECKLIST.md</strong></div>
        <div><span>设计规则</span><strong>relaycheck-desktop/DESIGN_SYSTEM.md</strong></div>
        <div><span>接力说明</span><strong>relaycheck-desktop/AGENT_HANDOFF.md</strong></div>
      </div>
      {showHelpGuide ? (
        <div className="detail-stack">
          <div className="problem-hint detail-hint">新手路径：先去"本机扫描"导入 NewAPI，再到"账号"补授权或 API Key，最后在"签到"和"余额"验证一次。</div>
          <div className="note">遇到异常优先看"总览"的处理建议中心；做批量操作前先在本页创建数据库备份。</div>
        </div>
      ) : null}
    </article>
  );
}

export function SettingsLegendCard() {
  return (
    <article className="card settings-legend-card">
      <div className="section-heading">
        <div>
          <strong>能力图例</strong>
          <span>常驻解释后台、Key、模型和价格 chip，减少状态只靠颜色判断。</span>
        </div>
      </div>
      <div className="chips">
        <span>NEW = NewAPI</span>
        <span>ONE = OneAPI</span>
        <span>SUB = Sub2API</span>
        <span>MOD = 魔改中转</span>
      </div>
      <div className="detail-list">
        <div><span>Key 有效</span><strong>已读取 /v1/models 且密钥可用</strong></div>
        <div><span>模型可用</span><strong>最小 chat completion 测试通过</strong></div>
        <div><span>raw_json</span><strong>来自 NewAPI 渠道原始配置的回退识别</strong></div>
        <div><span>live</span><strong>使用渠道 Key 实时请求上游模型列表</strong></div>
      </div>
    </article>
  );
}

export function SettingsSyncScheduleCard({
  syncSchedule,
  busy,
  canSave,
  onPatch,
  onSave,
}: {
  syncSchedule: SyncScheduleConfig;
  busy: boolean;
  canSave: boolean;
  onPatch: (patch: Partial<SyncScheduleConfig>) => void;
  onSave: () => void;
}) {
  return (
    <article className="card settings-sync-card">
      <div className="section-heading">
        <div>
          <strong>同步频率</strong>
          <span>默认每 30 分钟同步一次本地 NewAPI 数据；后台调度器会读取这里的配置。</span>
        </div>
        <span className={"status-pill " + (syncSchedule.enabled ? "success" : "neutral")}>
          <StatusLabel level={syncSchedule.enabled ? "enabled" : "disabled"} label={syncSchedule.enabled ? "已启用" : "未启用"} />
        </span>
      </div>
      <div className="proxy-toggle-row">
        <label className="check">
          <input type="checkbox" checked={syncSchedule.enabled} onChange={(event) => onPatch({ enabled: event.target.checked })} />
          启用定时同步
        </label>
        <label className="check">
          <input type="checkbox" checked={syncSchedule.runOnStartup} onChange={(event) => onPatch({ runOnStartup: event.target.checked })} />
          启动后同步一次
        </label>
      </div>
      <div className="proxy-form-grid">
        <label className="field">
          <span>同步间隔（分钟）</span>
          <input type="number" min={5} max={1440} value={syncSchedule.intervalMinutes} onChange={(event) => onPatch({ intervalMinutes: Math.max(5, Number(event.target.value) || 30) })} />
        </label>
        <label className="field">
          <span>同步模式</span>
          <select value={syncSchedule.mode} onChange={(event) => onPatch({ mode: event.target.value })}>
            <option value="local-newapi">本地 NewAPI 实例</option>
            <option value="manual-only">只手动同步</option>
          </select>
        </label>
      </div>
      <div className="problem-hint detail-hint">后台同步默认不导入渠道 Key、不做重探测，只更新渠道结构和源端移除状态；失败才发重要通知。</div>
      <div className="proxy-actions">
        <button disabled={busy || !canSave} onClick={onSave}>{busy ? "保存中…" : "保存同步频率"}</button>
      </div>
    </article>
  );
}

export function SettingsChannelHealthScheduleCard({
  channelHealthSchedule,
  busy,
  canSave,
  defaultConfig,
  onPatch,
  onSave,
}: {
  channelHealthSchedule: ChannelHealthScheduleConfig;
  busy: boolean;
  canSave: boolean;
  defaultConfig: ChannelHealthScheduleConfig;
  onPatch: (patch: Partial<ChannelHealthScheduleConfig>) => void;
  onSave: () => void;
}) {
  return (
    <article className="card settings-sync-card">
      <div className="section-heading">
        <div>
          <strong>渠道健康探测</strong>
          <span>定期刷新中转站识别、站点健康、渠道模型状态，并把异常推送到处理中心。</span>
        </div>
        <span className={"status-pill " + (channelHealthSchedule.enabled ? "success" : "neutral")}>
          <StatusLabel level={channelHealthSchedule.enabled ? "enabled" : "disabled"} label={channelHealthSchedule.enabled ? "已启用" : "未启用"} />
        </span>
      </div>
      <div className="proxy-toggle-row">
        <label className="check">
          <input type="checkbox" checked={channelHealthSchedule.enabled} onChange={(event) => onPatch({ enabled: event.target.checked })} />
          启用自动探测
        </label>
        <label className="check">
          <input type="checkbox" checked={channelHealthSchedule.runOnStartup} onChange={(event) => onPatch({ runOnStartup: event.target.checked })} />
          启动后立即探测
        </label>
        <label className="check">
          <input type="checkbox" checked={channelHealthSchedule.onlyRisky} onChange={(event) => onPatch({ onlyRisky: event.target.checked })} />
          只探测风险站点
        </label>
      </div>
      <div className="proxy-form-grid">
        <label className="field">
          <span>探测间隔（分钟）</span>
          <input type="number" min={5} max={1440} value={channelHealthSchedule.intervalMinutes} onChange={(event) => onPatch({ intervalMinutes: Math.max(5, Number(event.target.value) || 60) })} />
        </label>
        <label className="field">
          <span>单次站点上限</span>
          <input type="number" min={1} max={50} value={channelHealthSchedule.limit} onChange={(event) => onPatch({ limit: Math.min(50, Math.max(1, Number(event.target.value) || 20)) })} />
        </label>
      </div>
      <div className="problem-hint detail-hint">调度器会复用渠道页的“探测健康”流程，发现站点不可达、模型同步失败或 Key 状态异常时记录预警。</div>
      <div className="proxy-actions">
        <button disabled={busy || !canSave} onClick={onSave}>{busy ? "保存中…" : "保存健康探测计划"}</button>
        <Button variant="ghost" disabled={busy} onClick={() => onPatch(defaultConfig)}>恢复默认</Button>
      </div>
    </article>
  );
}

export function SettingsSchedulerCard({ scheduler, busy, onRefresh }: { scheduler: SchedulerStatus | null; busy: boolean; onRefresh: () => void }) {
  const checkinJob = scheduler?.jobs.find((job) => job.key === "checkin.daily");
  const syncJob = scheduler?.jobs.find((job) => job.key === "sync.local_newapi");
  const channelHealthJob = scheduler?.jobs.find((job) => job.key === "channel.health_probe");
  return (
    <article className="card scheduler-card">
      <div className="section-heading">
        <div>
          <strong>后台调度器</strong>
          <span>{scheduler ? ("状态刷新于 " + formatTime(scheduler.generatedAt)) : "读取自动签到和同步运行状态"}</span>
        </div>
        <Button variant="ghost" disabled={busy} onClick={onRefresh}>刷新</Button>
      </div>
      <div className="scheduler-job-grid">
        {[
          { key: "checkin.daily", fallback: "自动签到", job: checkinJob },
          { key: "sync.local_newapi", fallback: "NewAPI 定时同步", job: syncJob },
          { key: "channel.health_probe", fallback: "渠道健康探测", job: channelHealthJob },
        ].map(({ key, fallback, job }) => (
          <article className={"scheduler-job " + (job?.status || "idle")} key={key}>
            <div>
              <span>{job?.label || fallback}</span>
              <strong><StatusLabel level={job?.status || "idle"} label={schedulerStatusLabel(job?.status || "idle")} /></strong>
            </div>
            <div className="scheduler-job-meta">
              <span>下次 {formatTime(job?.nextRunAt || "")}</span>
              <span>上次 {formatTime(job?.lastFinishedAt || job?.lastStartedAt || "")}</span>
              {job?.summary ? <span>{job.summary}</span> : null}
              {job?.lastError ? <span className="danger-text">{job.lastError}</span> : null}
            </div>
          </article>
        ))}
      </div>
    </article>
  );
}

export function SettingsAuditLogCard({ auditLogs, busy, onRefresh }: { auditLogs: AuditLogItem[]; busy: boolean; onRefresh: () => void }) {
  return (
    <article className="card audit-log-card">
      <div className="section-heading">
        <div>
          <strong>审计日志</strong>
          <span>最近 {Math.min(auditLogs.length, 12)} 条安全与维护事件，只读留痕。</span>
        </div>
        <Button variant="ghost" disabled={busy} onClick={onRefresh}>刷新</Button>
      </div>
      <div className="list compact audit-log-list">
        {auditLogs.slice(0, 12).map((item) => (
          <article className={"detail-row audit-row " + item.level} key={item.id}>
            <div>
              <strong>{auditActionLabel(item.action)}</strong>
              <span>{item.summary} {"·"} {formatTime(item.createdAt)}</span>
            </div>
            <b><StatusLabel level={item.level} label={auditLevelLabel(item.level)} /></b>
          </article>
        ))}
        {!auditLogs.length ? <EmptyState title="暂无审计记录" description="登录、设置、备份、账号和站点维护会在这里留下只读记录。" /> : null}
      </div>
    </article>
  );
}

export function SettingsJsonEditor({
  settings,
  busy,
  onSave,
  onChange,
}: {
  settings: SystemSetting[];
  busy: boolean;
  onSave: () => void;
  onChange: (settings: SystemSetting[]) => void;
}) {
  return (
    <article className="card">
      <div className="section-heading">
        <div>
          <strong>系统设置 JSON</strong>
          <span>轻量保存扫描目标、签到计划和本地运行偏好。保存前会校验 JSON 格式。</span>
        </div>
        <button disabled={busy || !settings.length} onClick={onSave}>{busy ? "保存中…" : "保存设置"}</button>
      </div>
      <div className="settings-list">
        {settings.map((setting, index) => (
          <label className="settings-editor" key={setting.key}>
            <span>{setting.key} {"·"} 更新于 {formatTime(setting.updatedAt)}</span>
            <textarea
              value={setting.valueJson}
              onChange={(event) => {
                const next = [...settings];
                next[index] = { ...setting, valueJson: event.target.value };
                onChange(next);
              }}
            />
          </label>
        ))}
        {!settings.length ? <EmptyState title="正在读取设置" description="默认设置会在首次启动时自动初始化。" /> : null}
      </div>
    </article>
  );
}
