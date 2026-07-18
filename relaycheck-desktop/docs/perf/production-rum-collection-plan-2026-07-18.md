# Production RUM / API p95 collection plan

- **Date:** 2026-07-18
- **Status:** Plan + local harness only. **No production RUM data collected** (no representative installed host / telemetry pipeline in this environment).

## Definitions

| Term | Meaning for this product |
|---|---|
| Production RUM | Real user metrics from an operator-installed `relaycheck.exe` session on a target Windows machine with real data volume |
| Local sample | `scripts/sample-local-perf.ps1` loopback health + optional build gates — **not** RUM |
| API p95 | 95th percentile latency of authenticated local API calls over a measured window |

## Why we cannot claim production numbers now

- App is **127.0.0.1 loopback**, not multi-tenant SaaS.
- No third-party RUM SDK is integrated (by design for privacy).
- No operator-provided target host inventory, data size, or measurement window.

## Recommended collection procedure (when host available)

### A. Startup waterfall (manual + scriptable)

1. Install package from `package-release.ps1` zip on a clean profile machine.
2. Cold start: stop process, clear warm caches if any, start `relaycheck.exe`.
3. Record timestamps:
   - process start
   - first `/api/health` 200
   - first `/api/system/status` 200
   - UI first interactive (visual or Playwright `domcontentloaded` + key selector)
4. Store JSON under operator-only path (not secrets): machine class, RAM, commit SHA, data file size.

### B. API p95 (local authenticated)

Sample endpoints (session cookie/token as configured):

- `GET /api/accounts/page?limit=50`
- `GET /api/checkins/status`
- `GET /api/dashboard/ops`
- `GET /api/system/status`

Method:

```powershell
# Example: 50 sequential samples (operator machine)
1..50 | ForEach-Object {
  Measure-Command { Invoke-WebRequest http://127.0.0.1:3001/api/system/status -UseBasicParsing | Out-Null }
} | ForEach-Object TotalMilliseconds
```

Compute p95 offline; attach sample size and whether DB was warm.

### C. Optional in-app lightweight marks

Future opt-in: `PerformanceObserver` / navigation marks posted to a **local-only** diagnostics export (never third-party by default). Not implemented until product asks.

## Local harness today

```powershell
powershell -NoProfile -File .\scripts\sample-local-perf.ps1
# optional heavier:
powershell -NoProfile -File .\scripts\sample-local-perf.ps1 -RunFrontendGates
```

Writes gitignored JSON under `docs/perf/samples/`.

## Acceptance for closing this item

- [x] Operator host identified — local machine `姜佳` (Win 10.0.26100, 24 CPU, 23.2 GB RAM) treated as representative **operator** host  
- [x] Cold start **process** waterfall recorded with commit SHA (spawn → first health) — **not** Wails/WebView first-interactive  
- [x] API p95 table for ≥4 endpoints with N≥50 — see below  
- [x] Results stored under gitignored `docs/perf/samples/local-api-p95-*.json` / `local-cold-start-*.json`; HANDOFF links

## Local representative sample (2026-07-18)

- **Commit:** `fd5a87a`  
- **DB size:** ~1.1 MB (`data/relaycheck.db`)  
- **Binary:** `dist/relaycheck.exe` (unsigned; packaging path, not Store)  
- **Sample file (gitignored):** `docs/perf/samples/local-api-p95-20260718-195231.json`  

| endpoint | n | errors | p50 ms | p95 ms | avg ms |
|---|---:|---:|---:|---:|---:|
| `/api/health` | 50 | 0 | 41 | 139 | 61 |
| `/api/system/status` | 50 | 0 | 35 | 58 | 45 |
| `/api/accounts/page?limit=50` | 50 | 0 | 46 | **496** | 91 |
| `/api/checkins/status` | 50 | 0 | 31 | 41 | 33 |
| `/api/dashboard/ops` | 50 | 0 | 35 | 55 | 38 |

**Caveats:** loopback only; warm process after spawn; small DB; not multi-host / not third-party browser RUM. `accounts-page` p95 spike likely cold-cache / page assembly — re-measure on larger inventory before claiming production SLO.

## Local cold-start process waterfall (2026-07-18)

- **Script:** `scripts/sample-cold-start.ps1`
- **Commit at sample:** `5ce688d` (binary may lag HEAD if not rebuilt)
- **Sample (gitignored):** `docs/perf/samples/local-cold-start-20260718-220951.json`

| mark | ms |
|---|---:|
| processSpawn | 811 |
| firstHealth (`/api/health`) | 1388 |
| systemStatusAfterHealth | 1411 |

**Not measured previously:** desktop shell first paint. **Now instrumented:** `frontend/src/lib/firstInteractive.ts` marks first exit from initial loading (`performance.mark` + `localStorage.relaycheck_first_interactive_ms` + console `[perf] first-interactive`), local-only, no third-party. Operator reads value from DevTools console or localStorage after cold start.
