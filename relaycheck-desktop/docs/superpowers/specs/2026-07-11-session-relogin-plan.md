# #9 会话重登方案（签到 / 余额）

**日期：** 2026-07-11  
**状态：** 首期已实现（9.1–9.5 前端）· 阶段 2 仍搁置  
**范围：** 上游站账号登录态失效后的 **可重复人工闭环**（打开网页登录 → 保存授权 → 测登录态 → 签到/余额）  
**关联：**  
- 既有设计：`2026-07-03-login-flow-usable-loop-design.md`、`2026-07-04-checkin-login-execution-seam-design.md`  
- 布局 α：账号卡一级 CTA 已抬升「网页登录」  
- 代码锚点：`BrowserLoginService`、`AccountSessionService`、`AccountCard`、`accountActions.ts`、`TwoFactorGuide`  
- **实现：** `frontend/src/lib/accountActions.ts`（状态机 helper）· `AccountCard.tsx` · `accounts.css` 步骤条 · 单测

---

## 1. 目标与非目标

### 1.1 目标

让用户在 **auth 失效 / 需人工** 时，用最少迷糊步数完成：

```text
发现失效 → 打开正确登录入口 → 人工完成登录（含 2FA/验证码）
  → 保存浏览器授权 → 测试登录态 → 签到 / 刷新余额
```

验收强调：**稳定、可提示、可重复**；不是「替用户登录」。

### 1.2 非目标（硬禁止）

| 禁止项 | 说明 |
|--------|------|
| 自动填密码提交登录表单 | 出范围；安全与稳定性差 |
| 绕过 2FA / CAPTCHA / 短信邮箱 | 明确不做 |
| 新浏览器自动化框架 | 继续 Chrome + remote debugging 现有路径 |
| 在文档/日志写 Cookie、密码、profile 明文路径到对外材料 | 脱敏 |
| 与 #8 渠道 token 混为一谈 | 两套凭证 |

---

## 2. 现状能力（探查）

### 2.1 API（已存在）

| 方法 | 路径 | 角色 |
|------|------|------|
| POST | `/api/accounts/{id}/open-browser-login` | 开 Chrome profile + 登录 URL |
| POST | `/api/accounts/{id}/finish-browser-login` | 读 debug port Cookie → 加密入库 |
| POST | `/api/accounts/{id}/test-login` | 测登录态 |
| POST | `/api/accounts/{id}/checkin` | 签到 |
| POST | `/api/accounts/{id}/refresh-balance` | 余额 |
| POST | `/api/accounts/bulk-open-browser-login` 等 | 批量入口（Insights） |

### 2.2 后端服务缝（已抽）

| 服务 | 职责 |
|------|------|
| `BrowserLoginService` | Open / Save / ResolveTarget；状态 `opened` / `already_open` / `saved` / `missing` / `failed` |
| `AccountSessionService` | Ensure / LoginWithPassword / Save；密码多 path 尝试 |
| Session store | 内存 port/PID；进程退出清理 |

Open 时：`auth_type=browser_profile`，`login_status=manual_required`。  
Save 成功：`login_status=valid`，写 cookie 加密字段。

### 2.3 前端（α 后）

- 一级：`网页登录` · `签到` · `详情`  
- 更多：`保存授权` · `测试登录态` · `刷新余额` · 编辑 · 密钥 · `2FA 指引`  
- `loginStatus === two_factor_required` 时内联 `TwoFactorGuide`  
- 文案：`formatBrowserLoginOpenMessage` 明确「完成登录后点保存授权」；失效测态提示重登  

### 2.4 问题态集合

```text
PROBLEM_LOGIN_STATUSES: expired | manual_required | captcha_required | two_factor_required
PROBLEM_CHECKIN_STATUSES: auth_expired | manual_required | failed
```

`isProblemAccount` 驱动卡片高亮与筛选「异常」。

---

## 3. 缺口分析（#9 真正要补的）

| ID | 缺口 | 影响 | 优先级 |
|----|------|------|--------|
| R1 | 「保存授权」藏在更多里 | 重登主路径在 Open 后容易找不到第二步 | **P0** |
| R2 | 无「当前是否有打开的 browser session」UI 指示 | 用户不知能否点保存 | P0 |
| R3 | 失效后引导是分散的（状态字 / 2FA 条 / 更多菜单） | 步数与犹豫↑ | P1 |
| R4 | 密码登录 `Ensure` 与网页登录两条路径，失败时用户不知该走哪条 | 误点签到反复失败 | P1 |
| R5 | 批量 open 后 finish 节奏无向导 | 多账号重登累 | P2 |
| R6 | 登录入口低置信度时提示已有设计，落地完整度待核对 | 开错页 | P1（对齐 07-03 设计） |
| R7 | Action Center 可深链异常账号，但未强制展开重登向导 | 决策→执行断点 | P2 |

**布局 α 已做：** 把「网页登录」提到一级——#9 的 **半步**。  
**#9 本体：** 把 **Open → Save → Test → Checkin** 收成可感知状态机，而不是四个散按钮。

---

## 4. 方案选项

| 方案 | 内容 | 结论 |
|------|------|------|
| **A. 状态机条（推荐）** | 异常账号卡或详情顶：步骤条 1 打开 → 2 保存 → 3 测态 → 4 签到；按 `loginStatus` + 最近 action 结果点亮 | **采用** |
| **B. 仅文案** | 加 note「请先…」 | 不够 |
| **C. 全自动密码+Cookie** | 后台静默登录 | **拒绝**（2FA/ToS/稳定） |
| **D. 独立「重登」Tab** | 新一级导航 | 过重；否 |

---

## 5. 推荐产品行为

### 5.1 单账号重登状态机（UI）

```text
states:
  needs_login   ← expired | manual_required | captcha_* | two_factor_* | auth_expired(签到)
  browser_open  ← open-browser-login 成功且 session 未 save
  auth_saved    ← finish-browser-login saved
  auth_valid    ← test-login valid 或 loginStatus valid
  ops_ready     ← 可签到/刷余额
```

**按钮显隐建议：**

| 状态 | 一级强调 | 次级 |
|------|----------|------|
| needs_login | 网页登录 | 2FA 指引（若 two_factor） |
| browser_open | **保存授权**（升到一级，与网页登录并列或替换签到为次） | 取消/重开 |
| auth_saved | 测试登录态 | 签到 |
| auth_valid / ops_ready | 签到 · 详情 | 刷新余额在更多或一级旁 |

> 实现时可用本地组件 state：`lastBrowserOpenOk` + 服务端 `loginStatus`，不必新表。

### 5.2 与密码路径

- 若账号有可解密密码且站点支持 API 登录：`Ensure` 可在签到前尝试；失败则 **降级文案**：「自动登录失败，请网页登录并保存授权」。  
- 不在 UI 承诺「已自动登录成功」除非 test-login valid。

### 5.3 2FA / CAPTCHA

- 保持 `TwoFactorGuide`：说明在浏览器完成验证 → 回工具点保存授权。  
- 禁止任何「导出 TOTP / 跳过验证」类文案与功能。

### 5.4 批量

- 维持 bulk open / bulk finish；#9 首期只保证 **单卡状态机**。  
- 批量向导列为阶段 2（对齐 07-03 后续阶段 B）。

---

## 6. 实现切片（批准后编码）

| 切片 | 内容 | 主要文件（预期） |
|------|------|------------------|
| **9.1** | 异常卡：Open 成功后「保存授权」升一级直至 saved/关闭 | `AccountCard.tsx` |
| **9.2** | 步骤提示条 + `accountActions` 文案统一五类失败 | `AccountCard` / `accountActions.ts` |
| **9.3** | open 响应若已有 debug session，UI 显示「窗口已在运行」可保存 | 已有 status；接 UI |
| **9.4** | 签到/余额失败且 auth 类：卡片内快捷「去重登」滚到步骤条 | checkin 错误映射 |
| **9.5** | 单测：状态切换下按钮可见性；Go 测保持不泄密 | frontend tests |
| **9.6** | （可选）详情抽屉同步同一状态机 | `AccountDetailContent` |

**不改：** 加密方式、Chrome 启动参数骨架、路由 path 名称（除非缺字段再扩展响应）。

---

## 7. 错误语义（继承并收紧 07-03）

| 类型 | 用户下一步 |
|------|------------|
| 入口低置信度 | 确认是否后台登录页；可改站点登录 URL |
| 站点不可达 | 查 Base URL / 代理 |
| 人机验证 | 浏览器内完成后再保存 |
| 保存失败无 Cookie | 确认已登录成功再试；勿关窗口过早 |
| 测态 expired | 重新网页登录 |
| 无打开会话就保存 | 先点网页登录（status=missing） |

响应与通知：**禁止** Cookie 原文、Authorization、profile 绝对路径（若需调试仅本地 log 且默认关）。

---

## 8. 验收标准

| # | 标准 |
|---|------|
| 1 | 从 `expired` 账号：≤3 次主按钮完成 Open→Save→Test（不含人工在 Chrome 内操作时间） |
| 2 | Save 在「窗口已打开」后无需打开「更多」即可点到（9.1） |
| 3 | 2FA 账号见指引且能跳转 Open；无绕过文案 |
| 4 | test-login expired 文案指向重登而非「重试签到」空转 |
| 5 | 前端测 + 相关 Go 测绿；无新依赖 |
| 6 | smoke/mock 可演练 open/save/test 消息（真实 Chrome 手工） |

---

## 9. 与 α / β / #8

| 轨 | 关系 |
|----|------|
| 布局 α | 已提供一级「网页登录」；#9 补 Save 升权与状态机 |
| 布局 β | 主从不阻塞 #9；右栏账号卡共用同一状态机 |
| #8 | 渠道同步 token ≠ 账号会话；错误文案分离 |

---

## 10. 建议推进顺序

```text
评审本方案
  → 9.1 + 9.2（最大体感）
  → 9.3 + 9.4
  → 9.5 测试
  → 9.6 详情对齐（可选）
```

**阶段 2（另案）：** 批量重登向导、登录发现置信度 UX 全量、会话过期主动探测调度。

---

## 11. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-07-11 | 初稿：对齐现网 BrowserLogin/AccountCard 与 07-03 闭环设计；明确非自动登录；docs-only |
