# findings.md — 项目整理盘点

## 仓库状态（2026-07-18）
- Branch: `main` = `origin/main` @ `a8f372d`（工作树 clean 于整理前）
- 近 4 提交：
  1. `f5c10de` feat(security): previewId 闭环
  2. `b98218e` feat(frontend): typed API
  3. `5f5c556` docs: handoff/sop
  4. `a8f372d` chore: ignore verify canary

## 磁盘大户（未跟踪 / 可删）
| 路径 | 约大小 | tracked? | gitignore? | 决策 |
|---|---:|---|---|---|
| `dist/` | ~25.5 MB | 否 | 是 `dist/` | **删除** 发布二进制输出 |
| `frontend/dist/` | ~0.69 MB | 否 | 是 | **删除** Vite 构建 |
| `frontend/coverage/` | ~1.8 MB | 否 | 是 | **删除** vitest 覆盖率 HTML |
| `frontend/node_modules/` | 大 | 否 | 是 | **保留** 开发依赖 |
| `data/` | DB+keys | 否 | 是 | **禁止删** 运行时数据 |
| `vendor/` | 大 | 是 | 否 | **保留** Go vendor |
| `.planning/` | 多会话 md | 曾是 | 现是 | **已归档 tarball + 删除 + ignore** |
| `docs/` | 文档 | 是 | 否 | **保留** |

## 文档健康
- `docs/sop/*` 已含增量架构/QA/安全验证
- `HANDOFF.md` Last updated 仍写 2026-07-17，且含「uncommitted」与乱码（编码损坏段落）→ **需同步到 07-18 已推送状态**
- `README.md` / `CLAUDE.md` 已在 docs commit 中更新过

## 代码债务（本轮不修，仅记录）
- Settings 卡片组件覆盖率仍低；LocalNewAPISyncPanel 行为覆盖薄
- SitesPanel 等可能仍有裸 `/api/`（另任务）
- RUM/生产 p95 仍无法在仓内证明

## 安全
- 清理仅限 gitignore 构建产物
- 不触碰 `data/keys`、`instance.key`、真实 DB
