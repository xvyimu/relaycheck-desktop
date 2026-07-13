# HANDOFF.md

Authoritative handoff document for RelayCheck Desktop. Updated each session.
Read this first, then `CLAUDE.md` for architecture.

**Last updated:** 2026-07-13 (S4 + P2 三项 + P2 零散 done; committed local; **push 被权限系统拒绝，待授权**)

---

## ⏳ TODO (next session)

1. [ ] **Push**（被拒待授权）— 6 提交本地已就绪，origin/main 落后。上次 `git push` 被权限系统拒绝（外发动作）。授权后执行：`git push "ssh://git@github.com-obsidian/xvyimu/relaycheck-desktop.git" main`
2. [ ] **Optional re-release** — 推送后 `package-release` + `verify-package`（当前发布 zip 仍停在 `a611273`）

**已完成（勿重做）**

- [x] **P2 零散 (2026-07-13)** — 承 review BE-10 / BE-11 / AR-4  
  - **BE-10** Dashboard COUNT 聚合：`buildDashboardSummary` 5 次独立 `COUNT(*)` 折成单 SQL 标量子查询，省 4 次 SQLite RTT（`commit 459289e`）  
  - **BE-11** 错误稳定：`writeError` 已统一 `errorClass`（validation/server/rate_limited…）对外；`/api/v1` 前缀属契约级迁移（84 处调用点 + SPA 同步），单机本地形态无功能收益，**主动不做**  
  - **AR-4** 导航 intent：全局搜 `setTab("accounts"/"balances")` 字面量 **0 命中**，全走 `navigation.ts`（`accounts`→`{sites, accountsView:"all"}`、`balances`→`{sites, accountsView:"all", query:"余额"}`），已清  
  - Gates: core **55.4%** · go PASS · frontend **268** + tsc 0 + lint 0

- [x] **P2 三项落地 (2026-07-13)** — 承 review「Settings 大拆分 / 列表虚拟化 / 本机 API token」  
  - **Settings 大拆分**: `Settings.tsx` 789→431 行；抽出 `SettingsCards.tsx`（About/VersionCheck/PortCheck/Path/Help/Legend/Sync/ChannelHealth/Scheduler/AuditLog/JsonEditor 11 卡）；`parseSetting`/`parseStringSetting` 收敛 useMemo 解析  
  - **列表虚拟化（分页）**: `AccountsPanel.tsx` `ACCOUNTS_PER_PAGE=50` + `page` 状态 + `pageAccounts` 切片；筛选变化重置页；`pagination-bar.css`（>50 条才显示分页条）  
  - **本机 API token（opt-in）**: `internal/core/session_token.go`（256-bit hex + HttpOnly/SameSite=Strict cookie + `subtle.ConstantTimeCompare`）；`RELAYCHECK_REQUIRE_TOKEN=1` 才启用，token 写 `data/session-token.txt`(0600)；`requireSession` 前置 `validateSessionToken`；默认关闭不改现有可信单用户流；SPA 无需改动（`credentials: same-origin` 自动带 cookie）；`session_token_test.go` 4 用例  
  - Gates: core **55.2%** · go PASS · frontend **268** + lint 0 + tsc 0 + build 0
- [x] **S4 审查落地 (2026-07-13)** — 见 `docs/code-review-s4-implementation-2026-07-13.md`  
  - FE: dialogEpoch 关抽屉；safeExternalUrl；启动仅等 system；Insights 展开后拉 models  
  - BE: openAppDB；digest 单 cancel；health/status 路径脱敏；accounts `limit`；settings 白名单；JSON 8MiB；import 根收紧；releaseUrl sanitize  
  - CF: CI + go vet；`scripts/build-desktop.ps1`  
  - Gates: core **55.2%** · go packages PASS · frontend **268** + lint 0
- [x] **B · S3 (2026-07-13)**  
  - AR-1 freeze + cover ≥55；BE-3 loopback RemoteAddr；FE-4 cascade docs；CI 骨架
- [x] **A · Release zip (2026-07-13)** — commit `a611273`  
  - Zip: `dist/releases/relaycheck-desktop-1.1.0-a6112733657e-20260713-022651.zip`  
  - SHA256: `50d36aa7a6ea0ec387c1b3445cdfbf65ac1245c3c0b8f04b18caa1ea139808d5`
- [x] Push S0/S1/S2 — tip was `8b32f21` / S2 `91b9f40` → origin/main via `ssh://git@github.com-obsidian/xvyimu/relaycheck-desktop.git`（https 443 + 死代理 7897 仍不可用）
- [x] Visual smoke / session-expiry runbook / full-stack review report
- [x] S0 / S1 / S2 review（见 Current state）

**Push 备忘**

```powershell
cd E:\zidqiandao\relaycheck-desktop
git push "ssh://git@github.com-obsidian/xvyimu/relaycheck-desktop.git" main
# 勿依赖 https://github.com + 7897
```

---

## Current state

Layout optimization **alpha** is complete; **beta MVP (IA-1 master-detail)**, **IA-2** (accounts tab physically merged into 站点与账号), **#8.3 channel-sync UI**, **#9 Phase 2 bulk re-login**, **#8.4/#8.2**, **Action Center site sample deep-links**, **#9 R2 session indicator**, and **2026-07-12 frontend optimization + S0–S2 review** are on branch `main`.  
**Local tip:** `a611273` + **uncommitted S3+S4+P2**. **origin/main** behind until commit+push.

### P2 三项落地 (2026-07-13) — done

| 项 | 摘要 | 关键文件 |
|----|------|---------|
| Settings 大拆分 | 789→431 行；11 卡片抽出 | `frontend/src/components/settings/{Settings,SettingsCards}.tsx` |
| 列表虚拟化（分页） | 50/页；筛选重置；>50 才显示 | `AccountsPanel.tsx` · `styles/components/pagination-bar.css` |
| 本机 API token（opt-in） | `RELAYCHECK_REQUIRE_TOKEN=1`；HttpOnly/Strict cookie；常量时间比较；默认关 | `internal/core/session_token.go`(+`_test.go`) · `main.go` · `http.go:requireSession` |

Token 默认关闭；启用后 token 写 `data/session-token.txt`(0600)，`/api/health` 不受限，SPA 无需改动。

### S4 review implementation (2026-07-13) — done

See `docs/code-review-s4-implementation-2026-07-13.md`. Cover **55.2%**, frontend **268** tests.

### S3 mid-term (2026-07-13) — done

| ID | Summary | Status |
|----|---------|--------|
| AR-1 | App freeze policy + core cover ≥55% (55.1%) | done |
| BE-3 | Loopback RemoteAddr on writes + threat model doc; no unlock password | done |
| FE-4 | Recovery cascade docs + stub delete dates | done |
| CI | `.github/workflows/ci.yml` windows go/frontend | done |

### S2 review polish (2026-07-12) — done

| ID | Summary | Status |
|----|---------|--------|
| BE-5 | GET settings mask notification secrets; PUT empty preserves ciphertext | done |
| BE-4 | SQLite import path allowlist (Abs+EvalSymlinks+roots) | done |
| FE-2 | keep-alive `inert`/`aria-hidden`; `useApi({enabled})` pause | done |
| FE-3 | ChannelsPanel seeds from inventory; models-only when seeded | done |
| FE-6 | DialogShell body scroll-lock + `titleId`/aria-labelledby | done |
| FE-5 | SettingsProxy / SettingsBackup / SettingsExportImport; clear export password | done |
| FE-4 | recovery cascade note + drop duplicate `.muted` | done |
| FE-7 | settings-split + dialog-shell contract tests | done |

**Verify (S2):**

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/
cd frontend
npx tsc -b
npm test          # 264 passed
npm run lint
```

### Frontend optimization (2026-07-12) — done

Report: `docs/frontend-optimization-report-2026-07-12.md`

| Track | Summary | Status |
|-------|---------|--------|
| Theme integrity | Fix circular `--surface-solid`; class-only `html.dark`; FOUC bootstrap | done |
| Primitives | Tokenized Button/Card/Badge/Empty; **no Radix/shadcn** | done |
| DialogShell | Shared Escape + focus trap/cycle/restore; Sites / Accounts / master-detail / Channels / Onboarding / 2FA | done |
| Idle tabs | `lib/idle-tabs.ts` 5 min TTL; dashboard pinned | done |
| Lazy + chunks | Non-dashboard panels `React.lazy`; Vite `manualChunks` | done |
| CSS merge | `layers/control-room.css` = redesign → layout-harmonization → linear; old files stubbed | done |
| Button ghost | `variant="ghost"` → CSS class `ghost`; product ghost call sites migrated; 0 residual `button.ghost` natives | done |
| exhaustive-deps | Stable refresh/destructure + Settings `DEFAULT_*` module constants | done |
| Tooling | ESLint 9 flat + react-hooks; Prettier | done |

**Verify (frontend close-out):**

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npx tsc -b
npm test          # 264 passed
npm run build
npm run lint      # 0 errors, 0 warns
```

### Product tracks

| Track | Summary | Status |
|-------|---------|--------|
| S1–S6 | Action-first accounts/sites/dashboard/sidebar/CSS | done |
| β-MVP | `SiteAccountMasterDetail` dual-pane (≥1180) / stack (≤900); default layout on Sites | done |
| IA-2 | Physically merge accounts tab into 站点与账号 (`c517226`) | done |
| #8.3 | `LocalNewAPISyncPanel` + `syncFeedback`; Scan tab | done |
| #8.4 | exclude samples + `GET /api/local-newapi/exclude-rules` | done |
| #8.2 | last_sync_at / last_sync_summary on instances | done |
| #9 Phase 2 | `BulkReloginWizard` + detail re-login strip | done |
| #9 R2 / 9.3 | Session chip while `browser_open` | done |
| Action Center deep-link | `ActionSample` → sites master-detail | done |

### Constraints (still hold)

- No auto-login / 2FA bypass; never “请关闭 2FA”
- No secrets in docs/logs
- Reuse `?upstreamSiteId=`
- No Radix/shadcn (project-owned `components/ui/*` only)
- **Do not push unless asked** (or after proxy is up and user expects publish)

### Token save note (#8.3)

There is **no** `POST /api/local-newapi/{id}/sync-token`. Saving a system access token is done by posting `accessToken` + `saveAccessToken: true` on `POST …/{id}/sync`.

### Specs

- Frontend opt report: `docs/frontend-optimization-report-2026-07-12.md`
- **Full-stack review:** `docs/code-review-optimization-2026-07-12.md`（2026-07-12）
- β review: `docs/superpowers/specs/2026-07-11-layout-beta-design-review-draft.md`
- #8: `docs/superpowers/specs/2026-07-11-newapi-channel-sync-exploration.md`
- #9: `docs/superpowers/specs/2026-07-11-session-relogin-plan.md`

### Suggested verify after pull

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor -count=1 ./internal/accounts/ ./internal/core/
cd frontend
npx tsc -b
npm test
npm run lint
# optional: npm run build
```

Preserved: `vendor/`, `data/`, `frontend/dist/`. Run `cd frontend; npm ci` if `node_modules` missing.

---

## Historical sessions

See git log and earlier HANDOFF history for alpha S1–S6, AccountTaskService, CheckinTaskService, and global optimization closure.
