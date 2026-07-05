# 任务计划：移除全局调度幽灵站点

## 目标

把全局签到计划从 `upstream_sites` 虚拟记录中解耦：`channel_schedules` 允许全局计划用 `upstream_site_id = NULL` 存储，保留对旧 `__global__` 记录的兼容读取与迁移清理，避免站点列表、健康探测、统计和后续调度逻辑继续依赖幽灵站点。

## 当前阶段

- Phase 1：现状扫描与计划落盘 - complete
- Phase 2：RED 测试 - complete
- Phase 3：GREEN 实现 nullable 全局计划兼容迁移 - complete
- Phase 4：回归验证 - complete
- Phase 5：复查与交接 - complete

## 任务清单

- [x] 读取 ADR-002、上轮 `schedule-plan-projection` 计划、调度实现与测试。
- [x] RED：新增迁移/启动测试，证明新库和旧库都不再留下 `upstream_sites.id='__global__'`。
- [x] RED/CHECK：复用 schedule API / calendar / next-runs 现有测试，配合新增启动/迁移测试，证明全局计划仍出现但不是站点记录。
- [x] GREEN：重建 `channel_schedules` schema 为 nullable `upstream_site_id`，保留旧表数据。
- [x] GREEN：把 `EnsureGlobalScheduleRecord` / `SyncGlobalScheduleRecord` 改为 `id='__global__', upstream_site_id=NULL`，并删除旧虚拟站点。
- [x] GREEN：更新列表查询、投影、scheduler tick，正确识别全局计划。
- [x] REFACTOR：更新注释、项目结构文档和 ADR follow-up 状态。
- [x] CHECK：运行 focused 调度测试、`internal/core`、`internal/channels`、全量 Go、vet、前端相关测试/构建。
- [x] REVIEW：按 Standards/Spec 复查 diff。

## 范围边界

- 不改变公开 JSON 字段名；前端仍可看到全局计划项的 `upstreamSiteId="__global__"` 兼容标识。
- 不合并 `/api/scheduler/calendar` 和 `/api/scheduler/next-runs`。
- 不改变普通站点 schedule 的 `upstream_site_id` 语义。
- 不改变 `channel_accounts.upstream_site_id`。
- 不引入新的前端依赖。

## 风险与回滚

| 风险 | 缓解 |
|------|------|
| SQLite 无法直接修改 NOT NULL/FK | 采用安全表重建：关闭 FK、建新表、复制数据、替换表、重建索引、恢复 FK。 |
| 旧 `__global__` schedule 丢失 | 复制时把 `upstream_site_id='__global__'` 映射为 NULL，`id` 保持 `__global__`。 |
| 全局计划从列表中消失 | 查询层用 `CASE WHEN cs.id='__global__' THEN '__global__'` 输出兼容 ID 和站点名。 |
| 普通站点误接受 NULL | HTTP PUT 继续要求真实站点 ID；只有内部全局计划允许 NULL。 |
| 迁移后旧虚拟站点仍污染 UI | `EnsureGlobalScheduleRecord` 执行 `DELETE FROM upstream_sites WHERE id='__global__'`。 |

回滚方式：还原本次代码变更；生产数据回滚需使用发布前 `data\relaycheck.db` 备份。

## 验收标准

- 新建数据库启动后，`upstream_sites` 不包含 `__global__`。
- 旧数据库包含 `__global__` 虚拟站点和旧 schedule 时，启动迁移后站点被清除，schedule 保留。
- `/api/scheduler/channel-schedules` 仍返回一个全局计划兼容项。
- calendar 和 next-runs 仍包含全局签到计划。
- per-site scheduler tick 不处理全局计划，也不为 `NULL` site 执行签到。
- 全量 Go 测试、vet、前端测试/构建通过。
