# Frontend Optimization Report — 2026-07-12

RelayCheck Desktop (`relaycheck-desktop/frontend`)  
Constraint: **no Radix / no shadcn install** (project-owned `components/ui/*` only).

---

## 1. Scope decision

| Role prompt | Project hard constraint |
|-------------|-------------------------|
| “Integrate shadcn/ui” | `HANDOFF.md` / `DESIGN_SYSTEM.md`: **do not add Radix/shadcn** |

**Resolution:** treat “shadcn-like” goals as **token-driven local primitives + cascade cleanup**, not a library install.

---

## 2. What shipped

### P0 — Theme integrity

| Change | Path |
|--------|------|
| Fix circular `--surface-solid: var(--surface-solid)` → `#ffffff` | `frontend/src/styles/tokens.css` |
| Remove OS `@media (prefers-color-scheme: dark)` token mutation (manual light under dark OS works) | `frontend/src/styles/layers/radical-v4.css` |
| Dark tokens only via `html.dark` + `colorScheme` on apply | `theme-toggle.css`, `lib/theme.ts` |
| FOUC-safe bootstrap class | `frontend/index.html` inline script |

### P1 — Consistency / perf / cascade

| Change | Path |
|--------|------|
| `React.lazy` + `Suspense` for non-dashboard panels | `frontend/src/main.tsx` |
| Vite `manualChunks` for panel packages | `frontend/vite.config.ts` |
| Button / Card / Badge → pure `--v4-*` tokens (no `#hex` / slate / white) | `components/ui/{button,card,badge}.tsx` |
| Merge Empty + EmptyState API | `components/ui/empty.tsx` (+ re-export `empty-state.tsx`) |
| Move global `html.dark` chrome out of task-progress | `theme-toggle.css` ← from `task-progress.css` |
| Tokenize recovery surfaces; remove dead form stomps + unused `.login-card` / `.row-actions` | `layers/recovery.css` |
| Domain + finishing hex → v4 aliases (accounts/channels/checkins/settings/dashboard + layers) | `styles/domains/*`, `layers/*` |
| reduced-motion skeleton uses token bg; fix bad `pre` colors | `control-room-finishing.css` |
| Panel loading placeholder | `base.css` `.panel-loading` |
| Two-factor guide hex → v4 tokens; drawer base restored in drawers.css | `two-factor-guide.css`, `drawers.css` |

### P1 — A11y / keep-alive / tooling (session close-out)

| Change | Path |
|--------|------|
| Shared **DialogShell** (Escape, focus trap/cycle, restore focus, `role=dialog` `aria-modal`) — no Radix | `components/ui/dialog-shell.tsx` |
| Sites detail drawer → DialogShell (`detail-drawer-wide` via className) | `components/sites/SitesPanel.tsx` |
| Accounts / master-detail / channels drawers → DialogShell (removed local Escape + `drawer-backdrop`) | `AccountsPanel.tsx`, `SiteAccountMasterDetail.tsx`, `ChannelsPanel.tsx` |
| Onboarding wizard → DialogShell modal (`onboarding-overlay` / `onboarding-card` classes) | `components/onboarding/OnboardingWizard.tsx` |
| TwoFactorGuide `variant="dialog"` → DialogShell modal | `components/ui/TwoFactorGuide.tsx` |
| Pure idle tab policy module + 5 min TTL (dashboard pinned) | `lib/idle-tabs.ts`, `main.tsx` |
| ESLint 9 flat + `eslint-plugin-react-hooks` + Prettier | `eslint.config.js`, `.prettierrc.json`, `package.json` scripts |

### P2 — Optional debt closed (same session)

| Change | Path / note |
|--------|-------------|
| **Button ghost 批量迁** | `button.tsx` ghost → CSS class `ghost`（跳过 size，避免与 CR 层 min-height 冲突）；产品侧 ghost 工具栏统一 `<Button variant="ghost">`；原生 `<button className="…ghost…">` **0** 残留；主操作 / danger / `task-sample-link` 等非 ghost 仍原生 button |
| **exhaustive-deps 清理** | hooks：`useInventoryData` / `useModelUsageOverview` / `useOpsHealth` / `useSystemOverview` 解构 refresh；panels：`AccountsPanel` / `SiteAccountMasterDetail` / `ChannelsPanel` 稳定 deps；`Dashboard` schedulerJobs 收进 `useMemo`；`Settings` 模块级 `DEFAULT_PROXY_CONFIG` / `DEFAULT_SYNC_SCHEDULE` / `DEFAULT_CHANNEL_HEALTH_SCHEDULE` |
| **linear / CR 层合并** | 新建 `layers/control-room.css`：按历史 import 顺序拼接 **redesign → layout-harmonization → linear-calibration**；`styles.css` 单入口；旧三文件 stub 防双 import；**非**交互截图 QA（无运行时视觉回归自动化） |

### Tests

| Change | Path |
|--------|------|
| Assert `colorScheme` on applyTheme | `lib/__tests__/theme.test.ts` |
| DialogShell export contract | `components/ui/__tests__/dialog-shell.test.ts` |
| Idle tab prune policy | `lib/__tests__/idle-tabs.test.ts` |

---

## 3. Architecture snapshot (post-change)

- **Stack:** React 19 + Vite 8 + Tailwind v4 `@import` + plain CSS layers  
- **Nav:** tab state + `visitedTabs` keep-alive with **5 min idle TTL** (dashboard pinned)  
- **Theme:** class-only `html.dark`; system preference applied by JS (`initTheme` + head script)  
- **Overlays:** `DialogShell` for panel drawer + modal（产品抽屉/引导/2FA 全覆盖）  
- **Primitives:** project-owned Button/Card/Badge/Empty/DialogShell；ghost 走 CSS 类名以兼容 CR 层  
- **CSS order:** tokens → base → layout → domains → finishing → **`control-room.css`** → dashboard → radical-v4 → … → recovery → drawers  

---

## 4. Remaining debt

| Item | Status |
|------|--------|
| recovery.css wholesale delete | **Partial slim** — dead form stomps + unused classes removed; layout helpers kept |
| Merge linear-calibration + control-room-redesign + layout-harmonization | **Done (cascade-preserving file merge)** — 见 P2；真机 light/dark 视觉扫仍可选 |
| Domain hex → v4 | **Done** |
| ESLint/Prettier | **Done** |
| Dialog shell + all product drawers/modals | **Done** — Sites / Accounts / master-detail / Channels / Onboarding / 2FA |
| Idle tab unmount | **Done** (TTL 5 min; `lib/idle-tabs.ts`) |
| button.ghost → Button | **Done** — 0 residual ghost native buttons |
| exhaustive-deps structural cleanup | **Done** — lint 0 errors / 0 warns（本机 close-out） |

Optional later（不阻塞）:

1. 真机 light/dark + 抽屉/引导 截图级视觉 QA（form 已只靠 `base.css`）  
2. 主操作 / danger 是否再迁 Button（非 ghost，可选统一）  
3. recovery 布局 helper 是否再砍  

---

## 5. Verify (2026-07-12 close-out)

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
npx tsc -b        # OK
npm test          # 261 passed (27 files)
npm run build     # OK；CSS ~205.23 kB（`index-*.css`）
npm run lint      # 0 errors, 0 warns
```

Optional: open app, toggle light/dark with OS set opposite; open site/account/channel drawers + onboarding + 2FA (Tab cycle + Escape); leave a tab idle 5+ min and confirm remount on revisit.

---

## 6. Rollback

All changes under `frontend/` (and this report). **No git commit** unless you request one.

```powershell
cd E:\zidqiandao\relaycheck-desktop
git checkout -- frontend/ docs/frontend-optimization-report-2026-07-12.md
```

### Touched files (session)

- `frontend/index.html`
- `frontend/vite.config.ts`
- `frontend/package.json` (+ lockfile)
- `frontend/eslint.config.js`
- `frontend/.prettierrc.json`
- `frontend/.prettierignore`
- `frontend/src/main.tsx`
- `frontend/src/styles.css`（control-room 单入口）
- `frontend/src/lib/theme.ts`
- `frontend/src/lib/__tests__/theme.test.ts`
- `frontend/src/lib/idle-tabs.ts`
- `frontend/src/lib/__tests__/idle-tabs.test.ts`
- `frontend/src/styles/tokens.css`
- `frontend/src/styles/base.css`
- `frontend/src/styles/layers/radical-v4.css`
- `frontend/src/styles/layers/recovery.css`
- `frontend/src/styles/layers/control-room-finishing.css`
- `frontend/src/styles/layers/control-room.css`（**new** 合并层）
- `frontend/src/styles/layers/control-room-redesign.css`（stub）
- `frontend/src/styles/layers/layout-harmonization.css`（stub）
- `frontend/src/styles/layers/linear-calibration.css`（stub）
- `frontend/src/styles/domains/*` (hex → v4)
- `frontend/src/styles/components/theme-toggle.css`
- `frontend/src/styles/components/task-progress.css`
- `frontend/src/styles/components/two-factor-guide.css`
- `frontend/src/styles/components/drawers.css`
- `frontend/src/components/ui/button.tsx`（ghost → CSS class）
- `frontend/src/components/ui/card.tsx`
- `frontend/src/components/ui/badge.tsx`
- `frontend/src/components/ui/empty.tsx`
- `frontend/src/components/ui/empty-state.tsx`
- `frontend/src/components/ui/dialog-shell.tsx`
- `frontend/src/components/ui/TwoFactorGuide.tsx`
- `frontend/src/components/ui/__tests__/dialog-shell.test.ts`
- `frontend/src/components/sites/SitesPanel.tsx`
- `frontend/src/components/sites/SiteAccountMasterDetail.tsx`
- `frontend/src/components/accounts/*`（含 BulkRelogin / Detail / Card / Panel ghost 迁）
- `frontend/src/components/channels/*`（含 ChannelTable / ChannelsPanel）
- `frontend/src/components/dashboard/*`（Dashboard / HubRadar 等）
- `frontend/src/components/scan/LocalNewAPISyncPanel.tsx`
- `frontend/src/components/onboarding/OnboardingWizard.tsx`
- `frontend/src/components/settings/Settings.tsx`（DEFAULT_* 常量）
- `frontend/src/hooks/useInventoryData.ts` 等 exhaustive-deps 稳定化
- `docs/frontend-optimization-report-2026-07-12.md`

---

## 7. Decision log

- **No shadcn/Radix** — hard constraint wins over role prompt.  
- **Layer merge** = 保留历史 cascade 的单文件拼接 + stub 旧入口，不是删规则。  
- **Keep-alive TTL** = 5 minutes; dashboard always mounted.  
- **DialogShell width** = base `detail-drawer` only; callers pass `detail-drawer-wide` when needed (Sites/Channels).  
- **Button ghost** = CSS class `ghost` on primitive so CR layered selectors keep working; size utilities skipped for ghost.  
- **Lint** = react-hooks only; exhaustive-deps fixed structurally (stable fns / module defaults), not eslint-disable.  
- **No commit/push** unless requested.
