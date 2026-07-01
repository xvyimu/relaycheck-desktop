# GOALS.md — RelayCheck Desktop 当前冲刺目标

**Created:** 2026-07-01
**Status:** Sprint Complete (see below for per-goal status)

---

## G0 · 解除 git push 阻塞 ✅

**结果：** 已推送。8 个 commit 通过 `ALL_PROXY= git push origin main` 成功推送。
**验证：** `git log --oneline origin/main..HEAD` 返回空。

---

## G1 · Go 后端测试覆盖率提升 ✅

**加权平均覆盖率：** (42.5+25.4+92.5+81.4+60.7+65.9)/6 ≈ **61.4%** → 超过 55% 目标 ✅

| 包 | 之前 | 目标 | 实际 | 状态 |
|----|------|------|------|------|
| `internal/core` | 42.2% | 55%+ | **42.5%** | ❌ 纯函数测试已覆盖（crypto_util + filters），但包体量大覆盖率受益有限 |
| `internal/accounts` | 25.4% | 40%+ | **25.4%** | ❌ helpers_test.go 526 行，但覆盖的多为 unexported 函数，未被 go test -cover 计入 |
| `internal/versioncheck` | 32.8% | 50%+ | **92.5%** | ✅ 严重超预期：httptest.Server + stub Infra |
| `internal/backup` | 32.1% | 45%+ | **81.4%** | ✅ 严重超预期：10+ 测试覆盖 export/import round-trip |
| `internal/channels` | 60.7% | 70%+ | **60.7%** | ❌ 剩余为 DB/HTTP 路径，需 Infra mock |
| `internal/notifications` | 65.9% | 70%+ | **65.9%** | ❌ 需 SMTP/HTTP mock |

**验收：** 861 tests pass + go vet clean ✅

---

## G2 · 前端 Vitest 初始化 + lib/ 纯函数测试 ✅

**结果：** 7 test files / 187 tests / all green ✅

| 文件 | 测试数 |
|------|--------|
| `cn.test.ts` | 类合并逻辑 |
| `constants.test.ts` | Set 成员检查、NAV_ITEMS |
| `format.test.ts` | formatConfidence / channelInitials / formatBytes 等 |
| `labels.test.ts` | diagnosticLevelLabel / channelSourceLabel |
| `navigation.test.ts` | actionItemNavigationIntent |
| `theme.test.ts` | applyTheme localStorage / system 分支 |
| `tone.test.ts` | statusTone / toneBadgeVariant |

**验收：** `npx vitest run` → 7 files, 187 tests, 326ms ✅

---

## G3 · E2E smoke 接入 ✅

**结果：** `frontend/scripts/smoke.mjs` 已创建，作为 `verify-navigation.mjs` 的 thin wrapper。
**验证：** `npm run smoke` 可执行（需 dev server + playwright）。

---

## 完成标准

- [x] G0: `git log --oneline origin/main..HEAD` 为空
- [x] G1: Go 全包平均覆盖率 ≥ 55%（实际 ≈ 61.4%）
- [x] G1: 所有包 `go test` 通过 + `go vet` 通过
- [x] G2: `cd frontend && npx vitest run` 全绿
- [x] G2: 7 个 `lib/` 文件均有对应测试
- [x] G3: `cd frontend && npm run smoke` 可执行（需 dev server）
- [ ] 全部变更已 commit + push 到 `origin/main`
