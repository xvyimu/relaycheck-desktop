import type { ApiResult, ClientReadCacheEntry, GlobalApiError } from "@/types";

const clientReadCacheTTL = 1500;
const clientReadCache = new Map<string, ClientReadCacheEntry>();
const uncachedReadPrefixes = ["/api/checkins/status"];

const apiErrorListeners = new Set<(error: GlobalApiError) => void>();

class ApiError extends Error {
  errorClass?: string;
  status?: number;
  url: string;
  method: string;

  constructor(message: string, details: Omit<GlobalApiError, "message" | "occurredAt">) {
    super(message);
    this.name = "ApiError";
    this.errorClass = details.errorClass;
    this.status = details.status;
    this.url = details.url;
    this.method = details.method;
  }
}

export function subscribeApiErrors(listener: (error: GlobalApiError) => void) {
  apiErrorListeners.add(listener);
  return () => {
    apiErrorListeners.delete(listener);
  };
}

function publishApiError(error: GlobalApiError) {
  apiErrorListeners.forEach((listener) => listener(error));
}

function shouldCacheRead(url: string, method: string, options?: RequestInit) {
  return method === "GET" && !options?.body && !uncachedReadPrefixes.some((prefix) => url.startsWith(prefix));
}

function clearClientReadCache() {
  clientReadCache.clear();
}

export async function api<T>(url: string, options?: RequestInit): Promise<T> {
  const method = (options?.method || "GET").toUpperCase();
  const cacheable = shouldCacheRead(url, method, options);
  const cacheKey = `${method}:${url}`;
  const now = Date.now();
  if (cacheable) {
    const cached = clientReadCache.get(cacheKey);
    if (cached && cached.expiresAt > now) {
      return cached.promise as Promise<T>;
    }
  }

  const headers = options?.body ? { ...(options.headers as Record<string, string> | undefined), "content-type": "application/json" } : options?.headers;
  const request = fetch(url, {
    ...options,
    credentials: "same-origin",
    headers,
  }).then(async (response) => {
    const payload = (await response.json().catch(() => ({ ok: false, error: "响应不是有效 JSON。", errorClass: "bad_response" }))) as ApiResult<T>;
    if (!response.ok || !payload.ok) {
      const error = new ApiError(payload.error || "请求失败", {
        errorClass: payload.errorClass,
        status: response.status,
        url,
        method,
      });
      publishApiError({
        message: error.message,
        errorClass: error.errorClass,
        status: error.status,
        url,
        method,
        occurredAt: Date.now(),
      });
      throw error;
    }
    if (method !== "GET") {
      clearClientReadCache();
    }
    return payload.data as T;
  });

  if (cacheable) {
    clientReadCache.set(cacheKey, { expiresAt: now + clientReadCacheTTL, promise: request });
    request.catch(() => clientReadCache.delete(cacheKey));
  }

  return request;
}
