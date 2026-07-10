import { afterEach, expect, it, vi } from "vitest";

import { api } from "@/api/client";

afterEach(() => {
  vi.restoreAllMocks();
});

it("does not cache GET requests that carry an abort signal", async () => {
  let rejectFirst: ((error: Error) => void) | undefined;
  const url = `/api/health?case=${Date.now()}`;
  const fetchMock = vi
    .spyOn(globalThis, "fetch")
    .mockImplementationOnce(
      () =>
        new Promise<Response>((_, reject) => {
          rejectFirst = reject;
        }),
    )
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ ok: true, data: { status: "ok" } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

  const controller = new AbortController();
  const first = api<{ status: string }>(url, { signal: controller.signal }).catch((error) => error);
  const second = api<{ status: string }>(url);

  expect(fetchMock).toHaveBeenCalledTimes(2);
  await expect(second).resolves.toEqual({ status: "ok" });

  rejectFirst?.(new Error("signal is aborted without reason"));
  await first;
});
