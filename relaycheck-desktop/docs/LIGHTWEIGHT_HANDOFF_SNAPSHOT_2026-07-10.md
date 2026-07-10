# RelayCheck Desktop Lightweight Handoff Snapshot

Date: 2026-07-10

## State

The worktree is intentionally dirty and ready for a lightweight handoff. The goal is to keep source, docs, Go offline verification, and runtime data available while removing bulky generated frontend/release artifacts.

## Preserved

- `vendor/`: keep Go offline `-mod=vendor` test/build support.
- `data/`: real local runtime DB/key material; do not delete without backup and rollback.
- `frontend/dist/`: embedded frontend build required by `//go:embed frontend/dist`.

## Removed

- `dist/`: generated release binaries and historical packages; rebuild when needed.
- `frontend/node_modules/`: local npm install; restore with `cd frontend; npm ci`.
- `.tmp/`, `docs/reports/`, smoke output files, and TypeScript build info.

## Current Size Hotspots

- `vendor/`: about 223.85 MB.
- `data/`: about 5.05 MB.
- `frontend/`: about 1.29 MB after removing `node_modules`.

## Restore Commands

Run from `E:\zidqiandao\relaycheck-desktop`:

```powershell
cd frontend
npm ci
npm test
npm run build
cd ..
go vet -mod=vendor ./...
go test -mod=vendor -count=1 ./...
```

## Release Commands

Run only after deciding whether to commit or explicitly allow a dirty package build:

```powershell
cd E:\zidqiandao\relaycheck-desktop
cd frontend
npm ci
cd ..
powershell -ExecutionPolicy Bypass -File scripts\verify-release.ps1 -ProxyUrl http://127.0.0.1:7897
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-package.ps1
```

## Do Not Do By Default

- Do not delete `vendor/`.
- Do not delete or mutate `data/`.
- Do not package the historical `338870bc3154` release; it was cleaned and predates this optimization diff.
- Do not treat the current checkout as launch-ready until frontend dependencies are restored and release gates are rerun.
