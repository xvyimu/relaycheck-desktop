# RelayCheck Desktop Progress Report - 2026-06-24

## 1. Current Status

Status: P1 domain surface slice completed and committed.

Commit:

```text
a7ce0ba feat: add RelayCheck P1 domain surfaces
```

This slice upgrades RelayCheck Desktop from a recovered baseline into a more maintainable local operations console. The active domain pages for Sites, Check-ins, and Notifications have been split into dedicated frontend panels, smoke coverage now checks the primary tab surfaces on desktop and mobile widths, and the unsupported-check-in account cleanup flow is available through a dry-run-first UI and backend API.

The real `data/relaycheck.db` was not cleaned or directly edited.

## 2. Completed Work

### Frontend Domain Surfaces

Added dedicated React panels:

- `frontend/src/components/sites/SitesPanel.tsx`
- `frontend/src/components/checkins/CheckinsPanel.tsx`
- `frontend/src/components/notifications/NotificationsPanel.tsx`

`frontend/src/main.tsx` now imports and mounts these panels for the `Sites`, `Check-ins`, and `Notifications` tabs. This removes more page-specific logic from the main shell and makes later feature work safer.

### Smoke Coverage

Updated `frontend/scripts/smoke.mjs` to:

- Check the main tab surfaces on desktop.
- Re-check the same surfaces at 390px mobile width.
- Assert no horizontal overflow.
- Assert the Accounts page renders `.unsupported-cleanup-panel`.

### Unsupported Check-in Account Cleanup

Added backend route:

```text
POST /api/accounts/delete-unsupported-checkins
```

Implemented behavior:

- `dryRun=true` previews matched accounts without database writes.
- `dryRun=false` deletes matched accounts through the backend.
- Deletes related `checkin_logs` and `balance_snapshots`.
- Writes notification and audit entries only for real deletes.
- Supports `limit` and `includeLastUnsupported`.

Added Accounts UI cleanup panel in:

- `frontend/src/components/accounts/AccountInsights.tsx`

The UI shows matched/deleted counts, account samples, reason labels, and requires explicit confirmation before deletion.

### Detection Hardening

Strengthened upstream recognition in `internal/core/scanner.go`:

- NewAPI: `/api/about`, NewAPI-style check-in JSON fields.
- OneAPI: model/self signals without assuming check-in support.
- Sub2API: `/api/v1/*`, `/v1beta/models`, and gateway route signals.
- Disabled check-in text marks `supportsCheckin=false`.

Added/updated tests:

- `internal/core/accounts_cleanup_test.go`
- `internal/core/scanner_test.go`

## 3. Verification Results

All required checks passed on 2026-06-24.

```powershell
cd E:\zidqiandao\relaycheck-desktop\frontend
node --check scripts\smoke.mjs
npm run build
npm audit --audit-level=low
```

Result:

- Smoke script syntax: pass.
- Frontend production build: pass.
- npm audit: pass, `0 vulnerabilities`.

```powershell
cd E:\zidqiandao\relaycheck-desktop
go test -mod=vendor ./internal/core -run "TestDeleteUnsupportedCheckinAccounts|TestDetectUpstreamRecognizesNewAPICheckinStatusJSON|TestDetectUpstreamDoesNotSupportDisabledCheckin|TestDetectUpstreamRecognizesSub2APIGatewayRoutesWithoutBrandText"
go test -mod=vendor ./...
go build -mod=vendor -ldflags="-H windowsgui" -o dist\relaycheck.exe .
```

Result:

- Targeted cleanup/detection tests: pass.
- Full Go regression: pass.
- Windows GUI binary build: pass.

Browser smoke was run against a temporary runtime:

```text
http://127.0.0.1:3213
```

Result:

- Desktop tabs: pass.
- 390px mobile tabs: pass.
- Horizontal overflow: false.
- Accounts cleanup panel assertion: pass.

Sensitive scan result:

- No real credentials found in active source/docs.
- Only the deliberate fake API-key fixture in `internal/core/secrets_security_test.go` matched the token pattern.

## 4. Documentation Updated

Updated project tracking and handoff files:

- `task_plan.md`
- `progress.md`
- `findings.md`
- `AGENT_HANDOFF.md`
- `PRODUCT_RESEARCH_AND_REQUIREMENTS.md`

The plan now marks Phases 85, 86, and 87 complete.

## 5. Remaining Risks

- Real account cleanup has not been executed. It must follow: backup -> dry-run preview -> user confirmation -> API delete.
- Cleanup currently processes a limited batch. Large real datasets may need pagination or repeated batches.
- Upstream NewAPI/OneAPI/Sub2API APIs can change. Future detection rules should be tied to official source or captured real response samples.
- The repository root still contains unrelated untracked files outside `relaycheck-desktop`; they were intentionally left untouched.

## 6. Recommended Next Steps

1. Add cleanup history or a review log for real unsupported-check-in deletions.
2. Add seeded UI smoke data so the cleanup preview list is visually tested with matched accounts.
3. Add batch pagination for unsupported-check-in cleanup if real account counts exceed the current limit.
4. Continue reducing `frontend/src/main.tsx` by extracting the next stable page or workflow.

