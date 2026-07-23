# ChronoRelay

**ChronoRelay** is the public product name for **RelayCheck Desktop** — a local-first operations console for NewAPI / OneAPI / Sub2API and compatible relay sites.

Manage accounts, check-ins, balances, upstream detection, notifications, encrypted backups, and local NewAPI synchronization from a single desktop binary (Go + React + SQLite).

> Application source lives in [`relaycheck-desktop/`](./relaycheck-desktop/). Launch helpers at the repo root are Windows shortcuts into that app.

## Quick links

| | |
|---|---|
| App README | [relaycheck-desktop/README.md](./relaycheck-desktop/README.md) |
| License | [relaycheck-desktop/LICENSE](./relaycheck-desktop/LICENSE) (MIT) |
| Security | [SECURITY.md](./SECURITY.md) |
| Stack | Go `net/http` · React 19 / Vite · SQLite |

## Quick start

```bash
cd relaycheck-desktop/frontend && npm ci && npm run build && cd ..
go build -mod=vendor -o dist/relaycheck.exe .
./dist/relaycheck.exe
```

Open `http://127.0.0.1:3001`. See the app README for prerequisites and operator notes.

## Status

Solo-maintained product line under [@xvyimu](https://github.com/xvyimu). Default branch: `main`.
