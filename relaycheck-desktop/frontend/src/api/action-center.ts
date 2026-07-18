import { api } from "@/api/client";
import type { ActionSample } from "@/types";

export const actionCenterApi = {
  samples(actionId: string) {
    return api<ActionSample[]>(`/api/system/action-center/samples?id=${encodeURIComponent(actionId)}`);
  },
};
