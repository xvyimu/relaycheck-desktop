import { memo, useState } from "react";

import { actionCenterApi } from "@/api/action-center";
import { Button } from "@/components/ui/button";
import { actionItemNavigationIntent, actionSampleNavigationIntent } from "@/lib/navigation";
import type { ActionItem, ActionSample, NavigationIntent, TabKey } from "@/types";

type Navigate = (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;

function actionCategoryLabel(category?: string) {
  const labels: Record<string, string> = {
    auth: "授权",
    key: "Key",
    checkin: "签到",
    balance: "余额",
    channel: "渠道",
    health: "健康",
    site: "站点",
    notification: "通知",
    setup: "接入",
  };
  return labels[category || ""] || "运营";
}

function navigateAction(onNavigate: Navigate, item: ActionItem) {
  const { target, ...intent } = actionItemNavigationIntent(item);
  onNavigate(target, intent);
}

function navigateSample(onNavigate: Navigate, item: ActionItem, sample: ActionSample) {
  const { target, ...intent } = actionSampleNavigationIntent(item, sample);
  onNavigate(target, intent);
}

function ActionPriorityItemBase({ item, onNavigate }: { item: ActionItem; onNavigate: Navigate }) {
  const initialSamples = item.samples || [];
  const [samples, setSamples] = useState<ActionSample[]>(initialSamples);
  const [samplesOpen, setSamplesOpen] = useState(initialSamples.length > 0);
  const [samplesLoaded, setSamplesLoaded] = useState(initialSamples.length > 0);
  const [samplesLoading, setSamplesLoading] = useState(false);
  const [samplesError, setSamplesError] = useState("");
  const canLoadSamples = item.category !== "setup" && item.count > 0;
  const samplesId = `action-samples-${item.id}`;

  async function toggleSamples() {
    if (samplesOpen) {
      setSamplesOpen(false);
      return;
    }
    setSamplesOpen(true);
    if (samplesLoaded || samplesLoading) return;
    setSamplesLoading(true);
    setSamplesError("");
    try {
      setSamples(await actionCenterApi.samples(item.id));
      setSamplesLoaded(true);
    } catch (error) {
      setSamplesError(error instanceof Error ? error.message : "加载样本失败");
    } finally {
      setSamplesLoading(false);
    }
  }

  return (
    <article className={`dashboard-priority-item level-${item.level}`}>
      <div>
        <div className="dashboard-priority-head">
          <span className="action-category">{actionCategoryLabel(item.category)}</span>
          <b>{item.count}</b>
        </div>
        <strong>{item.title}</strong>
        <span>{item.impact || item.description}</span>
      </div>
      {samplesOpen ? (
        <div className="task-samples" id={samplesId} aria-live="polite">
          {samplesLoading ? <span>正在加载…</span> : null}
          {samplesError ? <span className="text-danger">{samplesError}</span> : null}
          {!samplesLoading && !samplesError && !samples.length ? <span>暂无样本</span> : null}
          {samples.slice(0, 3).map((sample, index) => {
            const clickable = Boolean(sample.entityType && sample.entityId);
            return clickable ? (
              <button
                key={`${sample.entityType}:${sample.entityId}:${sample.label}:${index}`}
                type="button"
                className="task-sample-link"
                onClick={() => navigateSample(onNavigate, item, sample)}
              >
                {sample.label}
              </button>
            ) : (
              <span key={`${sample.label}:${index}`}>{sample.label}</span>
            );
          })}
        </div>
      ) : null}
      <em>{item.recommendedAction || item.action}</em>
      <div className="dashboard-priority-actions">
        <button type="button" onClick={() => navigateAction(onNavigate, item)}>
          处理
        </button>
        {canLoadSamples ? (
          <Button
            variant="ghost"
            type="button"
            aria-expanded={samplesOpen}
            aria-controls={samplesId}
            onClick={() => void toggleSamples()}
          >
            {samplesOpen ? "收起样本" : "查看样本"}
          </Button>
        ) : null}
        <Button variant="ghost" type="button" onClick={() => navigateAction(onNavigate, item)}>
          查看列表
        </Button>
      </div>
    </article>
  );
}

export const ActionPriorityItem = memo(ActionPriorityItemBase);
