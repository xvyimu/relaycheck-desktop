# 手测记录：阶段 94-97

## 测试环境
- 操作系统：Windows
- 日期：2026-06-25
- 项目路径：e:\zidqiandao\relaycheck-desktop
- 本地 URL：http://127.0.0.1:3001

## 改前 vs 改后对照

### 1. 深色模式（T4.1）

**改前：** 深色模式下大量组件显示异常——白色背景面板、不可见文字、task-progress 状态徽章颜色不区分、ThemeToggle 使用 emoji 图标。

**改后：** 20 个 `--rc-*` 变量在 dark mode 中覆盖，200+ 行组件级深色规则，ThemeToggle 使用 SVG 图标（显示器/太阳/月亮）。切换深色模式后所有面板、输入框、徽章、图表均正确显示。

### 2. 版本检查 + 端口冲突（T6）

**改前：** 无版本检查功能；端口被占用时静默回退，用户不知情。

**改后：** Settings 页面新增版本检查卡片，可配置远程清单 URL 并检查更新。系统状态显示 `preferredPort` 和 `portConflict`，端口冲突时显示警告横幅。锁文件冲突时显示中文提示。

### 3. Webhook 重试

**改前：** Webhook 发送失败后直接返回错误，不重试。

**改后：** 指数退避重试（1s/2s/4s/8s/16s），最多 5 次。4xx 非 429 不重试，5xx 和网络错误自动重试。日志记录每次重试。

### 4. 加密 zip 导出/导入

**改前：** 无数据导出/导入功能。

**改后：** Settings 页面新增加密导出/导入卡片。AES-256-GCM 加密（RCZIP1 格式），包含 SQLite 数据库 + 全部设置 + manifest.json。导入时自动备份当前数据库，支持回滚。

### 5. 分析图表

**改前：** Dashboard 无数据分析图表。

**改后：** Dashboard 集成 AnalyticsPanel，包含：
- 余额趋势折线图（30 天，可切换 7/30/90 天）
- 签到状态分布环形图（7 天）
- API Key 响应时间条形图
- 站点可靠性表格
- 余额增量柱状图
- 点击数据点/环形图块可下钻查看详情

### 6. Cookie 过期追踪

**改前：** 无 Cookie 过期追踪。

**改后：** 保存 Cookie 时自动设置 30 天预估过期时间。诊断系统检测 7 天内即将过期的账号。Action Center 显示 Cookie 临近过期提醒。

### 7. 每渠道独立调度

**改前：** 仅有全局签到调度。

**改后：** 支持每站点独立配置签到时间和随机延迟。新增调度日历预览（7 天）和下次运行列表。

### 8. 桌面通知渠道

**改前：** 无桌面通知。

**改后：** 新增 desktop 通知渠道，通过站内通知表标记 `desktop-push`，前端 SSE 监听触发浏览器 Notification API。

### 9. 通知增强

**改前：** 通知仅支持全部标记已读和清除已读。

**改后：** 支持单条标记已读、按类型批量已读、按级别/类型/未读筛选通知列表。

### 10. Python 迁移 API

**改前：** Python 迁移器存在但无 HTTP API 端点。

**改后：** 新增 `POST /api/system/migrate-python-db` 端点，支持 dry_run 和 live 模式。

### 11. 2FA 登录指引

**改前：** 无 2FA 登录指引。

**改后：** 新增 TwoFactorGuide 组件，支持 inline 和 dialog 两种模式，显示 5 步操作指引和常见问题。集成到 AccountCard 和 AccountDetailContent。

### 12. Dry-run 预览

**改前：** 批量操作直接执行，无法预览。

**改后：** 新增 `POST /api/tasks/dry-run` 端点，预览批量签到/测试/识别操作，显示哪些账号将执行、哪些跳过及原因。

### 13. 开机自启

**改前：** 无开机自启功能。

**改后：** 新增 `GET/PUT /api/system/autostart` 端点，通过 PowerShell COM 创建 shell:startup 快捷方式。

### 14. 更新提示横幅

**改前：** 无更新提示。

**改后：** Dashboard 顶部显示 UpdateBanner，版本检查发现新版本时显示蓝色横幅，支持"查看更新"和"稍后提醒"。

### 15. 性能验证

**改前：** 无 500+ 行性能测试。

**改后：** 新增 `TestLargeDatasetPerformance` 测试，创建 500 个账号和 500 条签到记录，验证查询响应时间在可接受范围内（<500ms）。

### 16. 检测引擎测试

**改前：** 检测引擎无单元测试。

**改后：** 新增 4 个测试函数覆盖 header/HTML/API 响应检测和置信度计算，共 15+ 个测试用例。

### 17. 遗留检查 API

**改前：** 无遗留 Python 代码检查 API。

**改后：** 新增 `GET /api/system/legacy-check` 端点，检查遗留 Python api.py 路由数和 database.py 幂等性。

## 冻结项说明

以下 5 项为冻结项，不在本次范围内：

1. **relaycheck-hub 接入 node-cron 调度器** — relaycheck-hub 项目不存在于当前工作区
2. **relaycheck-hub 签到页接成真实功能** — 同上
3. **relaycheck-hub 余额页接成真实功能** — 同上
4. **newapi_signin/frontend/src/pages/Channels.jsx 拆为子组件** — 冻结遗留，不解冻
5. **newapi_signin/api.py 拆为 routers + detection** — 冻结遗留，不解冻

## 验证结果

- `go build ./...` — 通过
- `go test ./internal/core/ -count=1` — 全部通过（21s）
- `npm run build` — 通过（53 模块转换成功）
