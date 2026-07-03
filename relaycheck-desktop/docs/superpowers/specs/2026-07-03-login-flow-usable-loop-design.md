# 登录链路可用闭环设计

**日期：** 2026-07-03
**状态：** 待实施
**作者：** Codex brainstorming session

---

## 1. 背景

近期已完成登录入口识别的四个基础阶段：

- 新增登录发现类型与字段
- 扫描器产出结构化 `loginDiscovery`
- 站点创建、重识别、批量识别、详情加载时持久化登录元数据
- 站点详情抽屉展示登录来源、置信度和候选入口

下一步应把这些能力连成用户可实际完成的一条路径：从站点 URL 和账号信息出发，打开正确登录入口，保存授权，再验证签到和余额。

## 2. 目标

打通最小可用闭环：

```text
站点 URL / 账号信息
  -> 识别后台与登录入口
  -> 打开网页登录
  -> 用户完成登录
  -> 保存浏览器授权
  -> 测试登录态
  -> 触发签到 / 余额验证
```

验收重点不是自动替用户登录，而是让工具能稳定找到登录入口、打开正确页面、保存用户手动完成后的授权，并给出清晰的下一步动作和失败原因。

## 3. 候选方案

| 方案 | 内容 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A. 可用闭环优先 | 复用现有登录发现、网页登录、保存授权、测试登录、签到和余额能力，补齐入口选择、状态提示和回归测试 | 改动小，最快验证真实路径，风险低 | 自动化程度有限 | 采用 |
| B. 自动化增强闭环 | 在 A 上增加表单识别、跳转追踪、低置信度修正和失败诊断 | 能处理更多站点差异 | 范围较大，容易混入不稳定自动填表逻辑 | 后续阶段 |
| C. 全链路产品化 | 在 B 上增加批量向导、报告、完整测试矩阵和交接标准 | 产品体验完整 | 超出当前阶段，交付周期长 | 暂缓 |

本阶段采用方案 A，并把已发现的前端健壮性问题作为进入实现前的基础修复。

## 4. 范围

### 4.1 本阶段包含

- 修复站点详情抽屉的请求竞态，避免旧请求覆盖新详情。
- 校验 `loginDiscoveryJson` 的 shape，避免异常 JSON 造成前端渲染崩溃。
- 补齐登录识别候选入口和建议块的暗色主题样式。
- 后端网页登录入口选择使用稳定优先级：
  1. 用户手动指定的 `loginUrl`
  2. 高置信度 `loginDiscovery.url`
  3. 低置信度候选入口中的首选项
  4. 现有 fallback 登录路径
- 在账号或站点相关 UI 中串起动作链：
  - 打开网页登录
  - 保存授权
  - 测试登录态
  - 执行签到
  - 刷新余额
- 失败时返回可执行提示，例如低置信度、站点不可达、需要 2FA/CAPTCHA、保存授权失败、登录态测试失败。
- 增加覆盖此路径的自动化或半自动化验证。

### 4.2 本阶段不包含

- 不自动输入真实密码。
- 不绕过 2FA、验证码、短信或邮箱验证。
- 不把账号和签到领域做大规模架构拆分。
- 不做批量登录产品化。
- 不引入新浏览器自动化框架或新依赖。

## 5. 设计

### 5.1 登录入口选择

新增或收敛一个后端 helper，用于统一决定账号网页登录要打开的 URL。该 helper 的输出应包含：

- `url`：最终打开地址
- `source`：`manual` / `html_form` / `html_link` / `path_probe` / `spa_fallback` / `fallback`
- `confidence`：置信度
- `reason`：用于 UI 或日志的简短说明

选择策略：

1. 若站点存在手动 `loginUrl`，直接使用，`source=manual`，`confidence=1`。
2. 若 `loginDiscovery.url` 存在且置信度足够，使用该地址。
3. 若 `loginDiscovery.candidates` 存在，选择同源且看起来最像登录入口的第一个候选。
4. 否则沿用当前 fallback。

阈值建议：

- `confidence >= 0.6` 可自动用于打开。
- `confidence < 0.6` 仍可打开，但 UI 需要提示用户确认入口。

### 5.2 用户动作链

在不重做页面结构的前提下，保留当前安静、工具型后台风格，把动作链明确暴露在账号卡片或站点详情中：

```text
打开网页登录 -> 保存授权 -> 测试登录态 -> 签到 -> 刷新余额
```

动作按钮不应承诺“自动登录”。文案重点是“打开网页登录”和“保存已完成的授权”。

### 5.3 状态与错误

建议统一五类失败提示：

| 类型 | 示例 | 用户下一步 |
|------|------|------------|
| 入口低置信度 | 登录入口识别置信度偏低 | 打开页面后人工确认是否为后台登录页 |
| 站点不可达 | 目标站点连接失败或超时 | 检查 Base URL、代理和网络 |
| 人机验证 | 需要 2FA/CAPTCHA | 在浏览器中手动完成验证后再保存授权 |
| 授权保存失败 | 未发现有效 cookie/session | 确认浏览器已经登录成功后重试保存 |
| 登录态测试失败 | 已保存授权但接口仍未通过 | 重新打开网页登录或检查账号权限 |

错误提示应避免泄露 cookie、token、密码、Authorization header、API key 和浏览器 profile 路径。

### 5.4 前端健壮性

修复近期审查发现的问题：

- `openDetail` 使用 request sequence 或 AbortController，只接受最后一次详情请求结果。
- `parseLoginDiscovery` 做结构校验：
  - 必须是对象
  - `url`、`source` 为字符串时才使用
  - `confidence` 为有限数字时才使用
  - `candidates` 只有数组才保留，并过滤非字符串项
- 暗色主题下，候选入口和建议块使用现有 CSS 变量或 `html.dark` 覆盖。

### 5.5 后端接口

优先复用现有接口，不新增公开路径，除非实现中发现现有响应缺少必要诊断字段：

- `POST /api/accounts/{id}/open-browser-login`
- `POST /api/accounts/{id}/finish-browser-login`
- `POST /api/accounts/{id}/test-login`
- `POST /api/accounts/{id}/checkin`
- `POST /api/accounts/{id}/refresh-balance`
- `GET /api/upstream-sites/{id}`

如果需要给前端展示入口选择原因，优先扩展现有响应数据字段，而不是新增平行接口。

## 6. 数据流

```text
SitesPanel / AccountsPanel
  -> open-browser-login
      -> resolve login entry URL
      -> start or reuse browser session
      -> open browser at chosen URL
  -> finish-browser-login
      -> inspect browser session
      -> save sanitized authorization state
      -> update account login status
  -> test-login
      -> call authenticated endpoint
      -> return status and actionable reason
  -> checkin / refresh-balance
      -> run existing account operations
      -> update logs, snapshots, notifications
```

## 7. 测试策略

### 7.1 自动测试

- Go 单测：
  - 登录入口选择 helper 的优先级
  - 手动 login URL 不被自动发现覆盖
  - 低置信度 fallback 行为
  - 错误响应不包含敏感信息
- 前端单测或轻量测试：
  - `parseLoginDiscovery` 对异常 JSON 和异常 shape 不崩溃
  - 来源标签映射稳定

### 7.2 浏览器 smoke

用 mock API 覆盖：

1. 打开站点详情并展示登录入口。
2. 点击打开网页登录动作，验证请求发出且 UI 给出状态。
3. 模拟保存授权成功。
4. 模拟测试登录态成功。
5. 模拟签到或余额刷新动作成功。

真实浏览器登录依赖用户手动操作，自动 smoke 只验证 UI 和 API 编排，不放入真实凭据。

### 7.3 全量验证

实施完成后运行：

```powershell
rtk git diff --check
cd frontend; rtk npm run build
cd frontend; rtk npm test
rtk go test -mod=vendor -count=1 ./...
rtk go vet -mod=vendor ./...
rtk go build -mod=vendor ./...
```

## 8. 安全与隐私

- 不在日志、通知、错误响应、handoff、截图和测试 fixture 中写入真实凭据。
- 截图仅用于 mock 数据或本地人工验证，验证后清理。
- 浏览器授权保存只复用现有加密与脱敏机制。
- 任何返回给前端的诊断信息都只描述原因和下一步，不包含原始 cookie、token、header 或 profile 路径。

## 9. 验收标准

- 用户能从已有账号打开正确登录入口。
- 用户完成网页登录后，工具能保存授权并显示成功或可执行失败原因。
- 登录态测试能明确显示成功或失败原因。
- 可从同一工作流继续执行签到和余额刷新。
- 站点详情抽屉在连续点击、异常 `loginDiscoveryJson`、暗色模式下不产生明显错误。
- 前端构建/测试、Go test/vet/build 全绿。
- 不引入新依赖，不改变现有数据库 schema。

## 10. 后续阶段

本阶段完成后，再评估是否进入方案 B：

- 表单识别与跳转追踪
- 低置信度入口人工修正 UX
- 批量网页登录处理
- 更完整的失败诊断报告
- 组件级测试覆盖 `SitesPanel` 和账号动作链

---

**批准后下一步：** 将本设计转成分步实施计划，再开始编码。
