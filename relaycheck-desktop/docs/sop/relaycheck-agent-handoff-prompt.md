# RelayCheck 深度研究与推进——Agent 接手提示词

请直接复制以下提示词给新的主 Agent：

---

你将接手 `E:\zidqiandao\relaycheck-desktop` 的深度研究与增量推进工作。不要从零泛泛调研，也不要重复已经完成的产品阶段；请从现有增量 PRD 继续执行架构、工程实现和 QA 验证。

## 一、任务目标

深度核验 RelayCheck Desktop 的现有架构与关键数据流，在不做无依据大重构的前提下，推进一组最小风险、可验证、能直接提升操作者安全性和首次接入闭环的 P0/P1 改进。

目标项目：

- 工作区：`E:\zidqiandao`
- 项目目录：`E:\zidqiandao\relaycheck-desktop`
- 已完成的增量 PRD：`E:\zidqiandao\relaycheck-desktop\docs\sop\relaycheck-incremental-prd.md`

必须先完整读取该 PRD，并将它作为后续架构设计的输入。不得删除、覆盖或伪造其中的证据。

## 二、当前已确认的项目背景

RelayCheck Desktop 是供可信本机操作者使用的 NewAPI / OneAPI / Sub2API 本地运维控制台，采用单 Go 二进制、embedded React、SQLite 和 loopback HTTP 服务。它不是公开 SaaS 或多租户后台。

现有核心能力已经较完整：渠道、站点、账号、手工浏览器重登、签到、余额、任务、调度、Action Center、诊断、通知、分析、加密备份、CI 和 release gate 均有真实实现。项目处于“上线或长期运行前的闭环校准与证据补齐”阶段，不适合进行微服务化、数据库替换、全站 UI 重写或无性能证据的大规模重构。

产品阶段已发现三个有代码证据的交互断点：

1. `frontend/src/components/onboarding/OnboardingWizard.tsx` 的步骤 3仍残留“左侧账号页”旧文案；当前真实入口已经是“站点与账号 → 全部账号”。向导最后一步还可能直接启动签到，缺少安全预览。
2. 后端 `internal/core/dry_run.go` 已实现 `/api/tasks/dry-run`，但 `frontend/src/components/checkins/CheckinsPanel.tsx` 的“执行全部签到”仍直接启动任务，前端未形成 dry-run → 二次确认 → task start 的安全闭环。
3. `frontend/src/components/scan/ScanPanel.tsx` 导入成功后只有结果与刷新，缺少“查看渠道”和“前往站点与账号”的下一步导航。

最高风险点：dry-run 当前要求显式 `accountIds`，而批量 task start 可能只传 `type/params`。架构阶段必须证明预览候选集合与实际执行候选集合完全同源，避免出现“预览 A、执行 B”。在这个契约未确认前，禁止直接实现表面上的确认弹窗。

## 三、强制团队 SOP

你是交付主理人，只负责编排、核验和汇总，不得代写各专业角色的产出。

1. 先由主理人创建本次专项团队，建议名称：`software-relaycheck-handoff`。团队创建不得委派。
2. 本次产品阶段已经完成，不要重新生成 PRD。按照以下顺序继续：
   - 架构师：`name: "software-architect"`，`subagent_type: "software-architect"`
   - 工程师：`name: "software-engineer"`，`subagent_type: "software-engineer"`
   - QA：`name: "software-qa-engineer"`，`subagent_type: "software-qa-engineer"`
3. 必须顺序流转：架构师完整输出 → 主理人核验并转交工程师 → 工程师实现并通过一致性审查 → 主理人转交 QA。
4. 所有跨成员信息必须由主理人中转，成员不得互相直连。
5. 架构设计、代码实现、测试结论必须分别由对应成员产出；任何成员失败时，不得虚构其结果。
6. 为每个阶段建立并维护任务状态，完成一个阶段后立即标记完成。

## 四、架构师阶段

将增量 PRD 的完整内容和本提示词中的当前背景一并交给架构师。要求架构师亲自读取代码并输出：

- 现有架构与相关调用链的证据化审查；
- dry-run 与批量 task start 的真实候选账号来源、过滤规则、上限、竞态和一致性结论；
- 最小变更实现方案与框架选择；
- 精确文件列表及相对路径；
- 数据结构、TypeScript 类型、API 请求/响应契约；
- Mermaid 类图或模块关系图；
- Mermaid 时序图，至少覆盖 dry-run → 用户确认 → task start → 任务进度；
- 有序任务列表、依赖关系、实现顺序、回滚边界；
- 测试策略、依赖包变化和跨文件约定；
- 待明确事项以及是否需要退回产品阶段。

重点审查但不限于：

- `internal/core/dry_run.go`
- `internal/core/routes.go`
- 批量签到 task start 相关 handler、service、repository/query 文件
- `frontend/src/components/checkins/CheckinsPanel.tsx`
- `frontend/src/components/onboarding/OnboardingWizard.tsx`
- `frontend/src/components/scan/ScanPanel.tsx`
- `frontend/src/main.tsx`
- 现有 navigation helper、DialogShell、API client、任务 hooks 和相关测试

架构文档保存到：

`E:\zidqiandao\relaycheck-desktop\docs\sop\relaycheck-incremental-architecture.md`

默认产品决策：

- “执行全部签到”每次都必须 dry-run 并确认，不提供跨状态的“不再提示”。
- dry-run 必须与实际 task start 使用完全同源的候选集合；若现有 API 无法保证，应优先设计后端契约修正，而不是用前端猜测。
- Onboarding 优先导航或复用同一预览逻辑，不复制第二套后端规则。
- ScanPanel 成功导航默认打开目标面板，不自动添加可能隐藏新数据的过滤条件。
- `setupProgress` 和性能采集属于后续切片；除非 P0/P1 完成且风险很低，否则本轮不扩张范围。

若架构师确认 PRD 存在无法消除的关键歧义，必须停止工程阶段并明确返回产品经理；不得自行拍脑袋。

## 五、工程师阶段

主理人必须先阅读并理解架构文档，再把 PRD、架构设计、精确文件和任务顺序完整转交工程师。

工程师实施前必须：

1. 检查 `git status` 和目标文件 diff，识别已有未提交改动；
2. 只修改架构设计批准的文件，不清理、不覆盖其他人的工作；
3. 不触碰 `data/`、`vendor/`、`frontend/dist/`、真实凭据、数据库内容和用户个人文件；
4. 不新增大型运行时依赖，不降低测试或覆盖率阈值，不绕过 2FA/CAPTCHA，不泄露 token/cookie/password/API Key。

本轮优先实现：

### P0-A：Onboarding 契约修复

- 移除“左侧账号页”等幽灵入口，统一为“站点与账号 → 全部账号”。
- 最终步骤在未完成预览和确认时不得直接启动批量签到；优先引导至签到页或复用同一预览入口。
- “完成”只表示引导结束，不虚假表示全部账号已经验证成功。
- 补充真实交互测试，不只做静态字符串测试。

### P0-B：批量签到安全预览

- 点击“执行全部签到”后，严格执行：获取同源候选 → `/api/tasks/dry-run` → 展示将执行/跳过/原因 → 用户确认 → task start。
- 用户取消时 task start 请求数必须为 0。
- `willRun=0` 时禁用确认，并给出处理凭据或能力问题的下一步建议。
- dry-run 错误以稳定 `role=alert` 呈现，保留页面状态，允许重试；绝不静默降级为直接执行。
- 防止双击或忙碌态重复启动。
- 对超过 UI 展示上限的条目显示“另有 N 条”，同时遵守后端 200 账号上限。

### P1：扫描结果下一步导航

- 导入至少一项成功时展示“查看渠道”和“前往站点与账号”。
- 复用现有 `NavigationIntent` / `onNavigate`，不引入新路由库。
- 全失败时不展示误导性的成功导航；混合结果同时表达成功项和待处理项。
- 保持 390px 无横向溢出、可见焦点、44×44 触控目标和不只靠颜色表达状态。

工程师必须按项目 `package.json`、Go module 和 CI 中已定义的实际命令运行相关格式化、lint、typecheck、测试、coverage、build/budget，以及涉及后端时的 Go test/vet。不要猜命令，先读取现有脚本。

全部文件完成后必须执行全局一致性审查，并在输出中明确：

`IS_PASS: YES` 或 `IS_PASS: NO`

若为 NO，最多进行 2 轮修复与复审。只有 `IS_PASS: YES` 才能进入 QA。

## 六、QA 阶段

QA 必须以独立视角验证，不得只复述工程师测试结果。至少覆盖：

1. 批量签到确认路径：dry-run 先于 task start；确认后只启动一次。
2. 取消路径：task start 为 0 次。
3. 0 可执行路径：确认禁用，任务不启动。
4. dry-run 错误与重试路径：显示 `role=alert`，不危险降级。
5. dry-run 与 task start 候选集合契约：使用相同 fixture 或后端事实源证明同源。
6. Onboarding 四步切换、旧文案消失、最终步不越权启动、状态不串步。
7. ScanPanel 成功、混合、失败和两种导航行为。
8. 任务进度、取消和完成后刷新等既有行为不回退。
9. 前端 format/lint/typecheck/test/coverage/build/budget；涉及后端则运行 Go test/vet 和相应 smoke。
10. 不包含敏感信息；390px 布局和基础可访问性不回退。

QA 每轮都必须给出智能路由判定：

- `Engineer`：源码问题，附具体失败测试和文件位置，退回工程师修复；
- `QA`：测试本身问题，由 QA 自行修复；
- `NoOne`：全部通过。

最多 2 轮。第二轮仍失败时，必须明确遗留问题，不得声称完成。

QA 报告保存到：

`E:\zidqiandao\relaycheck-desktop\docs\sop\relaycheck-incremental-qa-report.md`

## 七、完成定义

只有满足以下全部条件，才可向用户报告交付完成：

- 架构文档已落盘，并证明或修正 dry-run 与 task start 候选集合一致性；
- P0/P1 目标代码已按批准范围完成；
- 工程师全局一致性审查为 `IS_PASS: YES`；
- QA 给出 `NoOne`，或明确披露第二轮后的遗留问题；
- 所有新增/修改文件、测试命令、通过率、失败项和已知风险有据可查；
- 不伪造真实目标机验收、RUM、启动 waterfall 或线上 API p95；
- 不代替人工操作者签署生产验收。

最终对用户汇报：

- TL;DR；
- 交付状态、测试通过率、已知问题数；
- 所有创建/修改文件路径；
- 关键架构决策；
- 未完成或必须人工执行的事项；
- 3-5 条下一步建议和准确启动/验证命令。

在整个执行过程中使用中文。遇到平台并发失败时应恢复同一成员上下文重试，不得把失败当成专业结论，更不得由主理人代写该成员产出。

---
