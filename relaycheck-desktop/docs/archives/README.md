# Planning history archive

- **Created:** 2026-07-18
- **Source:** `.planning/` (planning-with-files session outputs)
- **Archive:** `planning-history-2026-07-18.tar.gz` (31502 bytes)
- **Entries in tarball:** ~82 (files + directories)
- **Active plan at archive time:** `operator-monitor`

## Why archived

Historical agent planning sessions were tracked in git. They help archaeology but clutter the tree. Content remains in:

1. this tarball under `docs/archives/`
2. git history before the removal commit

## Restore

```bash
mkdir -p .planning
tar -xzf docs/archives/planning-history-2026-07-18.tar.gz -C .planning
```

```powershell
New-Item -ItemType Directory -Force .planning | Out-Null
tar -xzf docs/archives/planning-history-2026-07-18.tar.gz -C .planning
```

## Sessions included

- `account-cleanup-service`
- `account-creation-service`
- `account-login-batch-service`
- `account-session-cleanup-service`
- `account-site-update-service`
- `account-validation-service`
- `api-key-task-service`
- `browser-runtime-helpers`
- `final-launch-audit`
- `frontend-query-module`
- `launch-readiness-refresh`
- `operator-acceptance-record`
- `operator-launch-helper`
- `operator-launch-runbook`
- `operator-monitor`
- `release-package-manifest`
- `release-package-verifier`
- `schedule-global-record-removal`
- `schedule-plan-projection`
- `task-runner-boundary`
