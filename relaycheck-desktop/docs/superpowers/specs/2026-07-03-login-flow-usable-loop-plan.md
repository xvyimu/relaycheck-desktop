# Implementation Plan: 登录链路可用闭环

## Overview

本计划把 `2026-07-03-login-flow-usable-loop-design.md` 拆成小步实施任务。目标是先修复站点详情健壮性，再统一后端网页登录入口选择，最后把账号页动作链和验证补齐。每个任务都应保持系统可构建、可测试、可回退。

## Dependency Graph

```text
登录入口元数据（upstream_sites.login_url / login_discovery_json）
  -> accountAuthContext 加载字段
      -> 后端登录入口 resolver
          -> open-browser-login / bulk-open-browser-login
              -> 前端账号动作链
                  -> 浏览器 smoke / 全量验证

站点详情 UI 健壮性
  -> parse helper 可测试化
  -> drawer 请求竞态修复
  -> 暗色主题样式补齐
```

## Architecture Decisions

- 不新增公开 API 路径，优先扩展现有响应字段。
- 后端登录入口选择放在 `accountAuthContext` 附近，避免 `startBrowserLogin` 再做重复 SQL。
- 绝对登录 URL 不再无条件降级为 path；网页登录需要保留最终 URL，密码 API 登录仍只使用 API path candidates。
- 前端解析 `loginDiscoveryJson` 时把外部数据当作不可信输入，先规范化再渲染。
- 账号页不承诺自动登录，只展示“打开网页登录 -> 保存授权 -> 测试登录态 -> 签到/余额”的手动闭环。

## Task List

### Task 1: Harden site detail drawer

**Description:** 修复上轮审查发现的前端健壮性问题：详情请求竞态、`loginDiscoveryJson` shape 校验、候选入口和建议块暗色样式。

**Acceptance criteria:**
- [ ] 连续点击两个站点详情时，只展示最后一次点击的站点。
- [ ] `loginDiscoveryJson` 可解析但字段类型异常时，站点详情不崩溃。
- [ ] 暗色主题下候选入口和建议块不出现突兀浅色块。

**Verification:**
- [ ] `cd frontend; rtk npm test`
- [ ] `cd frontend; rtk npm run build`
- [ ] 手动或 Playwright mock 检查站点详情抽屉。

**Dependencies:** None

**Files likely touched:**
- `frontend/src/components/sites/SitesPanel.tsx`
- `frontend/src/styles.css`
- `frontend/src/lib/loginDiscovery.ts`
- `frontend/src/lib/__tests__/loginDiscovery.test.ts`

**Estimated scope:** Medium: 4 files

### Task 2: Add backend login entry resolver

**Description:** 在后端统一决定网页登录要打开的地址，并让 `open-browser-login` / `bulk-open-browser-login` 使用相同逻辑。保留现有密码 API 登录 path candidates，不把网页登录 URL 策略误用到 API 登录。

**Acceptance criteria:**
- [ ] 手动 `login_url` 优先，source 为 `manual`，confidence 为 `1`。
- [ ] 高置信度 `loginDiscovery.url` 可作为网页登录目标。
- [ ] 低置信度候选仍可打开，但响应中带出 reason 供前端提示。
- [ ] fallback 仍使用当前 `/login` 行为。
- [ ] 响应不泄露 cookie、token、Authorization、API key、profile path 以外的敏感值。

**Verification:**
- [ ] `rtk go test -mod=vendor -count=1 ./internal/core -run "LoginEntry|AccountAuth|BrowserLogin"`
- [ ] `rtk go test -mod=vendor -count=1 ./...`

**Dependencies:** Task 1 可并行，但建议先完成 Task 1 降低 UI 回归风险。

**Files likely touched:**
- `internal/core/checkin_balance.go`
- `internal/core/account_auth_repo.go`
- `internal/core/accounts.go`
- `internal/core/accounts_test.go`
- `internal/core/account_auth_repo_test.go`

**Estimated scope:** Medium: 5 files

### Task 3: Clarify account action chain

**Description:** 在账号卡片或账号详情中把现有动作组织成清晰闭环：打开网页登录、保存授权、测试登录态、签到、刷新余额。复用现有按钮与接口，重点改文案、状态信息和成功/失败反馈，不做大版式重设计。

**Acceptance criteria:**
- [ ] 用户能在一个账号卡片或详情面板中看到完整动作顺序。
- [ ] `open-browser-login` 成功后显示打开地址来源或可执行下一步。
- [ ] `finish-browser-login`、`test-login`、`checkin`、`refresh-balance` 成功后使用接口返回信息或明确状态文案。
- [ ] 2FA/CAPTCHA 场景继续引导用户手动完成验证，不承诺绕过。

**Verification:**
- [ ] `cd frontend; rtk npm test`
- [ ] `cd frontend; rtk npm run build`
- [ ] mock API 浏览器 smoke 覆盖动作链按钮。

**Dependencies:** Task 2

**Files likely touched:**
- `frontend/src/components/accounts/AccountCard.tsx`
- `frontend/src/components/accounts/AccountDetailContent.tsx`
- `frontend/src/components/accounts/helpers.ts`
- `frontend/src/types/index.ts`
- `frontend/src/styles.css`

**Estimated scope:** Medium: 5 files

### Task 4: End-to-end verification and handoff

**Description:** 补齐自动/半自动验证，记录验证结果和下一步 handoff。浏览器 smoke 使用 mock API，不写入真实凭据，不保留截图。

**Acceptance criteria:**
- [ ] mock 浏览器 smoke 覆盖站点详情展示和账号动作链。
- [ ] 全量前端与 Go 验证通过。
- [ ] 临时脚本和截图清理完成。
- [ ] 写入 Claude Code handoff，包含提交、验证结果、剩余风险。

**Verification:**
- [ ] `rtk git diff --check`
- [ ] `cd frontend; rtk npm run build`
- [ ] `cd frontend; rtk npm test`
- [ ] `rtk go test -mod=vendor -count=1 ./...`
- [ ] `rtk go vet -mod=vendor ./...`
- [ ] `rtk go build -mod=vendor ./...`

**Dependencies:** Tasks 1-3

**Files likely touched:**
- No production files required unless smoke reveals a regression.
- Temporary Playwright script under `%TEMP%`, removed before final commit.

**Estimated scope:** Small: verification and handoff only

## Checkpoints

### Checkpoint A: After Task 1

- [ ] Frontend build/test passes.
- [ ] Site detail drawer no longer has the three reviewed issues.
- [ ] No backend files touched unless required by a discovered compile issue.

### Checkpoint B: After Task 2

- [ ] Backend resolver tests pass.
- [ ] `open-browser-login` uses resolved URL.
- [ ] Password API login still uses API endpoint candidates.

### Checkpoint C: After Task 3

- [ ] UI exposes the complete usable loop without layout churn.
- [ ] Mock smoke can click through the action sequence.

### Checkpoint D: Complete

- [ ] All design acceptance criteria met.
- [ ] Full verification passes.
- [ ] Commit and push are atomic enough to review.
- [ ] Handoff memory updated without secrets.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Web login URL resolver accidentally affects password API login | High | Keep separate fields for browser target URL and API login path; test both |
| Old browser session reuse hides new URL choice | Medium | Include resolved URL/source in open response; document already-open behavior |
| Frontend action chain grows noisy | Medium | Reuse existing cards and compact hints; avoid new page-level layout |
| Real browser login cannot be fully automated | Medium | Mock UI/API flow in smoke; keep real-login steps manual |
| Sensitive values leak in messages | High | Reuse masking helpers; tests assert response/message does not contain secret fixtures |

## Open Questions

- None blocking. Current plan assumes existing API paths remain stable and no DB schema change is required.

## Implementation Order

1. Task 1: front-end hardening.
2. Task 2: backend login entry resolver.
3. Task 3: front-end action chain polish.
4. Task 4: verification, commit, push, handoff.

Each task should be committed separately if it produces a coherent, passing state. If the implementation stays small and tightly coupled, Tasks 1-3 may be one feature commit plus one verification/handoff commit.
