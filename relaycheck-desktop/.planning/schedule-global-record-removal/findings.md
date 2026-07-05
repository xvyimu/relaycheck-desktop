# 发现记录：移除全局调度幽灵站点

## 已确认现状

- ADR-002 明确把 `__global__` 虚拟 upstream site 作为后续 schema 决策。
- 当前 `NewApp` 启动后调用 `ensureGlobalScheduleRecord`，它会向 `upstream_sites` 插入 `id='__global__'` 的虚拟站点以满足 FK。
- `channel_schedules.upstream_site_id` 当前是 `TEXT NOT NULL`，并带 `FOREIGN KEY ... REFERENCES upstream_sites(id) ON DELETE CASCADE`。
- `channels.Service.ListChannelSchedules` 使用 `LEFT JOIN upstream_sites`，但因为 schema 非空/FK，全局记录仍必须有幽灵站点。
- `tickChannelScheduler` 已经显式跳过 `sched.ID == globalScheduleSiteID`，这部分行为可以保留。
- `sites.Service`、`core.sites.go`、`SiteTaskService` 已有查询过滤 `__global__` 的补丁，说明幽灵站点已经造成跨模块防御性代码。

## 设计判断

- 采用 nullable `channel_schedules.upstream_site_id` 是最直接的根治方式。
- 为避免前端和测试大面积改动，API 层仍输出 `upstreamSiteId="__global__"` 作为兼容标识。
- 迁移应在 `migrate()` 中完成，早于 `ensureGlobalScheduleRecord()`。
- 旧表迁移时需要 preserve `id`、enabled、time、cron、skip dates、delay、last/next run timestamps。

## 待验证

- SQLite 表重建在现有测试 helper 的 in-memory DB 中是否稳定。
- `PRAGMA foreign_keys` 默认状态与表重建步骤是否需要读取/恢复。
- 全局 schedule 是否出现在 `BuildNextRuns` 时需要特殊 label，避免空 SiteName 生成 `" 签到"`。

