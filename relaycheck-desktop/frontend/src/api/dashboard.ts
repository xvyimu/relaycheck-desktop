import { api } from "@/api/client";

/** Dashboard 聚合读取的 path 常量：useApi 消费者从这里取，禁止面板手写 URL。 */
export const DASHBOARD_OPS_PATH = "/api/dashboard/ops";
export const DASHBOARD_INVENTORY_PATH = "/api/dashboard/inventory";
export const DASHBOARD_MODEL_USAGE_PATH = "/api/dashboard/model-usage";

/** 直接 GET 用的 typed helper（当前 hooks 走 useApi(path)；helper 供任务/测试复用）。 */
function ops<T>(): Promise<T> {
  return api<T>(DASHBOARD_OPS_PATH);
}

function inventory<T>(): Promise<T> {
  return api<T>(DASHBOARD_INVENTORY_PATH);
}

function modelUsage<T>(): Promise<T> {
  return api<T>(DASHBOARD_MODEL_USAGE_PATH);
}

export const dashboardApi = {
  opsPath: DASHBOARD_OPS_PATH,
  inventoryPath: DASHBOARD_INVENTORY_PATH,
  modelUsagePath: DASHBOARD_MODEL_USAGE_PATH,
  ops,
  inventory,
  modelUsage,
} as const;
