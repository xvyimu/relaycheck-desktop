# RelayCheck Workspace

当前主项目是 `relaycheck-desktop/`。除非任务明确点名旧实现，后续开发、测试、文档和构建都以这个目录为准。

## 根目录入口

| 路径 | 用途 |
|------|------|
| `relaycheck-desktop/` | 正式桌面版源码：Go 后端、React/Vite 前端、SQLite、本地控制台。 |
| `启动RelayCheck.bat` | 启动 `relaycheck-desktop/dist/relaycheck.exe`，默认打开浏览器。 |
| `静默启动RelayCheck.vbs` | 静默启动 `relaycheck-desktop/dist/relaycheck.exe`，不自动打开浏览器。 |
| `_archive/2026-06-24-workspace-cleanup/` | 已归档的旧 Python、旧 Vite、Next 实验项目和旧工作文档。 |
| `data/` | 历史/本地数据目录；清理或迁移前必须先备份。 |

## 活跃项目

进入正式项目：

```powershell
cd E:\zidqiandao\relaycheck-desktop
```

核心文档：

- `relaycheck-desktop/README.md`
- `relaycheck-desktop/docs/PROJECT_STRUCTURE.md`
- `relaycheck-desktop/PRODUCT_RESEARCH_AND_REQUIREMENTS.md`
- `relaycheck-desktop/task_plan.md`
- `relaycheck-desktop/progress.md`
- `relaycheck-desktop/AGENT_HANDOFF.md`

阶段报告已整理到：

- `relaycheck-desktop/docs/reports/P0_PROGRESS_20260623.md`
- `relaycheck-desktop/docs/reports/PROJECT_PROGRESS_REPORT_2026-06-24.md`

## 清理边界

- 不直接删除或修改 `relaycheck-desktop/data/relaycheck.db`。
- 不把真实密码、Cookie、Token、API Key 写入源码、文档、日志或截图。
- `dist/`、`frontend/dist/`、`frontend/node_modules/`、`.pipeline/smoke-runtime*` 属于生成或运行产物，不作为源码维护。
- 旧实现已经归档；如需恢复，从 `_archive/2026-06-24-workspace-cleanup/` 移回即可。
