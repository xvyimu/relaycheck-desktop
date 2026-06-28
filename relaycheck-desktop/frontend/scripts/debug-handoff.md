# RelayCheck 延续提示词 — NavigationIntent 传播端到端调试

## 当前环境

| 组件 | 状态 | 地址 |
|------|------|------|
| Go 后端 | ✅ 运行中 | `127.0.0.1:3001`（`dist/relaycheck.exe`） |
| Vite dev server | ✅ 运行中 | `127.0.0.1:5173`（API 代理 → `:3001`） |
| 浏览器 | ✅ 已打开 | `http://127.0.0.1:5173` |

## 未提交的改动

- `relaycheck-desktop/.gitignore` — 添加了 `frontend/npm-dev.log` 条目
- 需要 `git add relaycheck-desktop/.gitignore && git commit -m "chore: ignore npm-dev.log"`（可选，不紧急）

## 已完成的工作

### NavigationIntent 传播链路（已提交 `8a91882`）

```
HubRadar/Dashboard 点击"处理"
    → actionItemNavigationIntent(item) in navigation.ts
    → onNavigate(target, intent)
    → main.tsx handleNavigate() setTab + setNavigationIntent
    → 面板 useEffect 读 intent 设 filter
    → useMemo 过滤数据
```

### 各面板 intent 处理

| 面板 | intent 字段 | 效果 | 状态 |
|------|------------|------|------|
| AccountsPanel | `accountStatus: "problem"` | 问题账号前置排序 | ✅ |
| ChannelsPanel | `siteHealth: "risk"` / `sourceStatus` / `channelKind` | 筛选健康风险/缺失/未知 | ✅ |
| CheckinsPanel | `checkinStatus: "problem"` → 映射为 `"failed"` | 筛出失败日志 | ✅ |
| NotificationsPanel | `unreadOnly: true` | 仅显示未读 | ✅ |

### 已确认数据

Action Center 返回 6 条待办：
1. `auth-required-accounts` → accounts, filter=problem
2. `today-checkin-problems` → checkins, filter=problem
3. `balance-missing` → accounts, filter=all
4. `unknown-channels` → channels, filter=unknown
5. `missing-channels` → channels, filter=missing
6. `unread-notifications` → notifications, filter=unread

## 待验证（需要人为在浏览器中点一点）

1. **Dashboard 加载** — 确认 UI 渲染、HubRadar 显示 6 条待办
2. **签到异常的"处理"按钮** → 跳转到 CheckinsPanel，筛选出失败日志
3. **失效授权的"处理"** → AccountsPanel 问题账号前置
4. **未知渠道的"处理"** → ChannelsPanel 自动筛选 `upstreamKind: "unknown"`
5. **缺失渠道的"处理"** → ChannelsPanel 筛选 sourceStatus: "missing"
6. **未读通知的"处理"** → NotificationsPanel 仅显示未读
7. **余额缺失的"处理"** → AccountsPanel 无筛选（filter=all）

## 疑点记录

- `lsof` 在 Bash 环境不可用（exit 127），端口清理改用 PowerShell
- 此前 Vite 有端口冲突（5173/5174/5175 被占用），清理后已正常
- 前端 `/api/system/action-center` 通过 Vite proxy 转发到 `:3001`，已验证 200

## 后端进程管理

Go 后端 PID 可通过 `Get-Process -Name relaycheck` 查看。
Vite 进程可通过 `Get-Process -Name node` 查看。
停止命令：`Get-Process -Name relaycheck | Stop-Process` / `Get-Process -Id <nodePID> | Stop-Process`