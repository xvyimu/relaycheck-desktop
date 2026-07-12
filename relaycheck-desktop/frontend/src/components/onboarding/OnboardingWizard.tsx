import { useEffect, useRef, useState } from "react";
import { api } from "@/api/client";
import { DialogShell } from "@/components/ui/dialog-shell";
import { LineIcon } from "@/components/ui/line-icon";
import type { LineIconName } from "@/types";
import { Button } from "@/components/ui/button";

const ONBOARDING_FLAG = "relaycheck_onboarding_done";
const REOPEN_EVENT = "relaycheck:reopen-onboarding";

type StepKey = "connect" | "channels" | "credentials" | "checkin";

interface StepMeta {
  key: StepKey;
  index: number;
  icon: LineIconName;
  title: string;
  description: string;
}

const STEPS: StepMeta[] = [
  {
    key: "connect",
    index: 1,
    icon: "sites",
    title: "连接 NewAPI",
    description: "填入 NewAPI 后台地址和访问令牌，工具会自动导入渠道结构。",
  },
  {
    key: "channels",
    index: 2,
    icon: "channels",
    title: "导入渠道",
    description: "去渠道页操作导入，或直接在这里触发一次模型同步。",
  },
  {
    key: "credentials",
    index: 3,
    icon: "accounts",
    title: "配置凭据",
    description: "去「站点与账号」页的「全部账号」子视图，为每个站点补充登录凭据或 API Key。",
  },
  {
    key: "checkin",
    index: 4,
    icon: "checkins",
    title: "试签到一次",
    description: "触发一次签到任务，验证整条链路是否畅通。",
  },
];

export function OnboardingStepIcon({ name }: { name: LineIconName }) {
  return (
    <div className="onboarding-step-icon" aria-hidden="true">
      <LineIcon name={name} />
    </div>
  );
}

export function onboardingStatusProps(kind: "success" | "danger") {
  return kind === "danger"
    ? ({ role: "alert", "aria-live": "assertive", "aria-atomic": true } as const)
    : ({ role: "status", "aria-live": "polite", "aria-atomic": true } as const);
}

interface ImportFromAdminResult {
  instanceId?: string;
  importedCount?: number;
  sitesCreated?: number;
  sitesMerged?: number;
  detectedCount?: number;
  syncTokenSaved?: boolean;
}

interface ChannelModelSyncOverview {
  total?: number;
  synced?: number;
  failed?: number;
  items?: Array<{ channelId?: string; channelName?: string; status?: string; message?: string }>;
}

interface TaskStartResult {
  taskId: string;
}

function isOnboardingDone() {
  try {
    return window.localStorage.getItem(ONBOARDING_FLAG) === "1";
  } catch {
    return false;
  }
}

function markOnboardingDone() {
  try {
    window.localStorage.setItem(ONBOARDING_FLAG, "1");
  } catch {
    /* ignore */
  }
}

function clearOnboardingDone() {
  try {
    window.localStorage.removeItem(ONBOARDING_FLAG);
  } catch {
    /* ignore */
  }
}

export function reopenOnboarding() {
  clearOnboardingDone();
  window.dispatchEvent(new CustomEvent(REOPEN_EVENT));
}

export function OnboardingWizard() {
  const [open, setOpen] = useState(false);
  const [stepIndex, setStepIndex] = useState(0);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const baseUrlInputRef = useRef<HTMLInputElement>(null);

  // Step 1 form state
  const [baseUrl, setBaseUrl] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [saveToken, setSaveToken] = useState(true);

  useEffect(() => {
    if (!isOnboardingDone()) {
      setOpen(true);
    }
    function handleReopen() {
      setStepIndex(0);
      setMessage("");
      setError("");
      setBaseUrl("");
      setAccessToken("");
      setSaveToken(true);
      setOpen(true);
    }
    window.addEventListener(REOPEN_EVENT, handleReopen as EventListener);
    return () => {
      window.removeEventListener(REOPEN_EVENT, handleReopen as EventListener);
    };
  }, []);

  useEffect(() => {
    if (!open || stepIndex !== 0) return;
    const frame = window.requestAnimationFrame(() => baseUrlInputRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [open, stepIndex]);

  function close() {
    markOnboardingDone();
    setOpen(false);
  }

  function next() {
    if (stepIndex < STEPS.length - 1) {
      setStepIndex(stepIndex + 1);
      setMessage("");
      setError("");
    } else {
      close();
    }
  }

  function skip() {
    setMessage("");
    setError("");
    next();
  }

  async function runStep() {
    const step = STEPS[stepIndex];
    setBusy(true);
    setMessage("");
    setError("");
    try {
      if (step.key === "connect") {
        if (!baseUrl.trim() || !accessToken.trim()) {
          setError("请填写 NewAPI 后台地址和访问令牌。");
          setBusy(false);
          return;
        }
        const result = await api<ImportFromAdminResult>("/api/local-newapi/import-from-admin-api", {
          method: "POST",
          body: JSON.stringify({
            baseUrl: baseUrl.trim(),
            accessToken: accessToken.trim(),
            saveAccessToken: saveToken,
            importKeys: false,
            skipCreateSites: false,
            detectAfterImport: false,
          }),
        });
        setMessage(
          `已导入 ${result.importedCount ?? 0} 个渠道，新建站点 ${result.sitesCreated ?? 0} 个，合并站点 ${result.sitesMerged ?? 0} 个。`,
        );
      } else if (step.key === "channels") {
        const result = await api<ChannelModelSyncOverview>("/api/channels/models/sync", {
          method: "POST",
          body: JSON.stringify({ limit: 10 }),
        });
        setMessage(`模型同步完成：共 ${result.total ?? 0} 个，成功 ${result.synced ?? 0} 个，失败 ${result.failed ?? 0} 个。`);
      } else if (step.key === "credentials") {
        setMessage("已记录。请稍后到「站点与账号」页的「全部账号」子视图，为每个站点补充登录凭据或 API Key。");
      } else if (step.key === "checkin") {
        const result = await api<TaskStartResult>("/api/tasks/start", {
          method: "POST",
          body: JSON.stringify({ type: "checkin", params: {} }),
        });
        setMessage(`已触发签到任务，任务编号 ${result.taskId}。可在「签到」页查看进度。`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "操作失败，请稍后重试。");
    } finally {
      setBusy(false);
    }
  }

  const step = STEPS[stepIndex];
  const isLast = stepIndex === STEPS.length - 1;
  const canRun =
    step.key !== "connect" || (baseUrl.trim().length > 0 && accessToken.trim().length > 0);

  return (
    <DialogShell
      open={open}
      onClose={close}
      variant="modal"
      ariaLabel="首次启动引导"
      backdropClassName="onboarding-overlay"
      className="onboarding-card"
      initialFocusSelector={stepIndex === 0 ? "input[type='url']" : undefined}
    >
      <header className="onboarding-header">
        <div className="onboarding-title">
          <span className="onboarding-brand">RelayCheck</span>
          <span className="onboarding-eyebrow">首次启动引导</span>
        </div>
        <div className="onboarding-steps" role="list" aria-label="步骤指示器">
          {STEPS.map((item, idx) => (
            <span
              key={item.key}
              role="listitem"
              aria-current={idx === stepIndex ? "step" : undefined}
              aria-label={`步骤 ${item.index}/${STEPS.length}：${item.title}${idx < stepIndex ? "，已完成" : ""}`}
              className={
                "onboarding-step-dot" +
                (idx === stepIndex ? " active" : "") +
                (idx < stepIndex ? " completed" : "")
              }
              title={`${item.index}/${STEPS.length} ${item.title}`}
            >
              {item.index}
            </span>
          ))}
        </div>
      </header>

      <div className="onboarding-body">
        <div className="onboarding-step">
          <OnboardingStepIcon name={step.icon} />
          <div className="onboarding-step-text">
            <div className="onboarding-step-meta">
              步骤 {step.index}/{STEPS.length}
            </div>
            <h3 className="onboarding-step-title" id="onboarding-title">{step.title}</h3>
            <p className="onboarding-step-desc">{step.description}</p>
          </div>
        </div>

        {step.key === "connect" ? (
          <form
            className="onboarding-form"
            onSubmit={(event) => {
              // Allow Enter-to-submit from the text inputs. The visible
              // "执行" button lives outside this form and is type="button",
              // so without this handler + hidden submit input the form's
              // onSubmit would be dead code.
              event.preventDefault();
              if (!busy && canRun) {
                void runStep();
              }
            }}
          >
            <label className="onboarding-field">
              <span>NewAPI 后台地址</span>
              <input
                ref={baseUrlInputRef}
                type="url"
                inputMode="url"
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
                placeholder="https://your-newapi.example.com"
                autoComplete="url"
                autoCapitalize="none"
                spellCheck={false}
              />
            </label>
            <label className="onboarding-field">
              <span>访问令牌（Access Token）</span>
              <input
                type="password"
                value={accessToken}
                onChange={(event) => setAccessToken(event.target.value)}
                placeholder="NewAPI 后台 -> 个人设置 -> 访问令牌"
                autoComplete="new-password"
                autoCapitalize="none"
                spellCheck={false}
              />
            </label>
            <label className="onboarding-check">
              <input
                type="checkbox"
                checked={saveToken}
                onChange={(event) => setSaveToken(event.target.checked)}
              />
              保存令牌以便后续定时同步
            </label>
            <button
              type="submit"
              aria-hidden="true"
              tabIndex={-1}
              style={{ position: "absolute", width: 1, height: 1, padding: 0, margin: -1, overflow: "hidden", clip: "rect(0,0,0,0)", border: 0 }}
            />
          </form>
        ) : null}

        {step.key === "channels" ? (
          <div className="onboarding-hint">
            你可以前往左侧「渠道」页查看导入的渠道，或在这里点击「执行」触发一次模型同步。
          </div>
        ) : null}

        {step.key === "credentials" ? (
          <div className="onboarding-hint">
            前往左侧「账号」页为每个站点补充登录凭据或 API Key。本步骤无需在此执行操作。
          </div>
        ) : null}

        {step.key === "checkin" ? (
          <div className="onboarding-hint">
            点击「执行」触发一次签到任务，验证账号凭据和站点规则是否就绪。
          </div>
        ) : null}

        {message ? <div className="onboarding-status success" {...onboardingStatusProps("success")}>{message}</div> : null}
        {error ? <div className="onboarding-status danger" {...onboardingStatusProps("danger")}>{error}</div> : null}
      </div>

      <footer className="onboarding-footer">
        <Button variant="ghost" type="button" onClick={skip} disabled={busy}>
          跳过
        </Button>
        <div className="onboarding-footer-actions">
          {step.key !== "credentials" ? (
            <button
              type="button"
              onClick={() => void runStep()}
              disabled={busy || !canRun}
            >
              {busy ? "执行中…" : "执行"}
            </button>
          ) : null}
          <button type="button" onClick={next} disabled={busy}>
            {isLast ? "完成" : "下一步"}
          </button>
        </div>
      </footer>
    </DialogShell>
  );
}
