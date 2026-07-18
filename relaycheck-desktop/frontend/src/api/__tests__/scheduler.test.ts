import { afterEach, describe, expect, it, vi } from "vitest";

import { schedulerApi } from "../scheduler";

afterEach(() => {
  vi.restoreAllMocks();
});

function mockOk(data: unknown = {}) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify({ ok: true, data }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    ),
  );
}

describe("schedulerApi", () => {
  it("list/calendar/nextRuns 使用固定只读路径", async () => {
    const fetchMock = mockOk({ generatedAt: "t", items: [] });
    await schedulerApi.listChannelSchedules();
    await schedulerApi.calendar();
    await schedulerApi.calendar(14);
    await schedulerApi.nextRuns();

    expect(fetchMock).toHaveBeenCalledWith("/api/scheduler/channel-schedules", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/scheduler/calendar?days=7", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/scheduler/calendar?days=14", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/scheduler/next-runs", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("saveChannelSchedule 原样提交声明表单字段", async () => {
    const fetchMock = mockOk({ ok: true });
    const form = {
      upstreamSiteId: "site-1",
      enabled: true,
      checkinTime: "08:00",
      cronExpr: "",
      skipDates: ["2026-07-20"],
      randomDelayMin: 0,
      randomDelayMax: 30,
    };
    await expect(schedulerApi.saveChannelSchedule(form)).resolves.toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledWith("/api/scheduler/channel-schedules", {
      method: "PUT",
      body: JSON.stringify(form),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });
});
