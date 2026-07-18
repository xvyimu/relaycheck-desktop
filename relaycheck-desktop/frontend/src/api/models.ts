import { api } from "@/api/client";
import type { ModelOverview, ModelPricingOverview } from "@/types";

export type ModelSyncOptions = {
  limit?: number;
};

export type PricingSyncOptions = {
  limit?: number;
};

/** 读取本地模型覆盖概览；只读 GET，不触发上游探测。 */
function getModelOverview(): Promise<ModelOverview> {
  return api<ModelOverview>("/api/models/overview");
}

/** 读取本地价格缓存概览。 */
function getPricingOverview(): Promise<ModelPricingOverview> {
  return api<ModelPricingOverview>("/api/models/pricing");
}

/** 同步 Key 模型能力并返回最新概览；默认 limit=50，仅发送声明字段。 */
function syncModels(options: ModelSyncOptions = {}): Promise<ModelOverview> {
  return api<ModelOverview>("/api/models/sync", {
    method: "POST",
    body: JSON.stringify({ limit: options.limit ?? 50 }),
  });
}

/** 探测上游价格写入本地缓存；默认 limit=50，仅发送声明字段。 */
function syncPricing(options: PricingSyncOptions = {}): Promise<ModelPricingOverview> {
  return api<ModelPricingOverview>("/api/models/pricing/sync", {
    method: "POST",
    body: JSON.stringify({ limit: options.limit ?? 50 }),
  });
}

export const modelsApi = {
  overview: getModelOverview,
  pricing: getPricingOverview,
  sync: syncModels,
  syncPricing,
} as const;
