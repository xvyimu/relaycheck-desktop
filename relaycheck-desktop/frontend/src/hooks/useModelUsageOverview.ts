import { useCallback } from "react";

import { useApi } from "@/hooks/useApi";
import type { ModelOverview, ModelPricingOverview, UsageOverview } from "@/types";

export function useModelUsageOverview() {
  const model = useApi<ModelOverview | null>("/api/models/overview", null);
  const pricing = useApi<ModelPricingOverview | null>("/api/models/pricing", null);
  const usage = useApi<UsageOverview | null>("/api/usage/overview", null);
  const { refresh: refreshModel } = model;
  const { refresh: refreshPricing } = pricing;
  const { refresh: refreshUsage } = usage;

  const refresh = useCallback(async () => {
    await Promise.all([refreshModel(), refreshPricing(), refreshUsage()]);
  }, [refreshModel, refreshPricing, refreshUsage]);

  return {
    modelOverview: model.data,
    pricingOverview: pricing.data,
    usageOverview: usage.data,
    loading: model.loading || pricing.loading || usage.loading,
    loaded: model.loaded && pricing.loaded && usage.loaded,
    error: model.error || pricing.error || usage.error,
    refresh,
  };
}

export type ModelUsageOverviewState = ReturnType<typeof useModelUsageOverview>;
