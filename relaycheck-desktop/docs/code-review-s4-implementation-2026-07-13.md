# RelayCheck Desktop · S4 审查落地说明（2026-07-13）

承接 `docs/code-review-optimization-2026-07-12.md` 与 2026-07-13 全栈审查报告。  
**状态：S4a–S4c 代码已落地并通过本地闸门（未要求则不自动 commit/push）。**

## 闸门

| 项 | 结果 |
|----|------|
| `go test ./internal/{core,accounts,notifications,versioncheck}` | PASS |
| `go test -cover ./internal/core` | **55.2%**（≥55） |
| frontend `tsc` + vitest | **268** PASS |
| `npm run lint` | 0 error |

## 已落地

### 前端

| ID | 项 | 实现要点 |
|----|----|----------|
| FE-A | 跨 tab Dialog 滚动锁 | `main.tsx` `dialogEpoch`；Channels/Sites/Settings 在 epoch 变化时关抽屉/详情 |
| FE-B | `releaseUrl` 校验 | `lib/safeExternalUrl.ts` + UpdateBanner/Settings + 单测 |
| FE-C | 冷启动减负 | `appIsInitialLoading` 仅等 system；`useModelUsageOverview({enabled: system.loaded})` |
| FE-D | Insights 懒拉 models | `AccountInsights` 仅 `expanded` 后请求 overview/pricing |
| FE-E/F 部分 | Settings 可接 epoch；safe URL | 编排大拆分未强行切碎（风险/收益） |

### 后端

| ID | 项 | 实现要点 |
|----|----|----------|
| BE-A | restore DB 池 | `openAppDB` 统一 NewApp/reopen；repo + hub `SetDB` |
| BE-B | multi-digest cancel | 单一 parent `digestCancel` 管全部 loop |
| BE-C | 路径脱敏 | health Message=`ok`；status `DatabasePath`/`BackupDir` 用 basename |
| BE-D | 账号列表 | `?limit=` 默认 500 max 1000；缓存键含 limit |
| BE-E | settings 白名单 + body 上限 | `isAllowedSystemSettingKey`；`MaxBytesReader` 8MiB |
| BE-F | 导入根收紧 | 默认去掉 UserHome/APPDATA；保留 `RELAYCHECK_SQLITE_IMPORT_ROOTS` |
| BE-G | 外链 | versioncheck `sanitizeReleaseURL` |

### 配置

| 项 | 说明 |
|----|------|
| CI | `.github/workflows/ci.yml` + `go vet` + cover 地板 |
| `scripts/build-desktop.ps1` | npm build + ldflags go build 一键 |

## 有意未做 / 延后

- 完整 Settings 状态机拆到 &lt;300 行（工作量大，S2 已拆 UI 片）
- 账号列表虚拟化 UI
- 可选本机 API token（产品仍为本机信任）
- 自动 git commit/push（需你确认）
- 重打 release zip（工作区脏时可 `-AllowDirty` 或先提交）

## 验证命令

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/ ./internal/notifications/ ./internal/versioncheck/
go test -mod=vendor -count=1 -coverprofile=core-cover.out ./internal/core/
cd frontend; npx tsc -b; npm test; npm run lint
```
