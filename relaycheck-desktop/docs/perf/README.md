# Local performance sample (not production RUM)

- **Date:** 2026-07-18
- **Scope:** local desktop loopback only. Does **not** prove production RUM/API p95.
- **How to regenerate:** `powershell -NoProfile -File scripts/sample-local-perf.ps1`

## What this is

A lightweight local harness that:

1. Optionally hits `GET /api/health` against a running RelayCheck instance.
2. Records wall-clock timings for a small set of **local** commands (frontend test/build when requested).
3. Writes a JSON sample under `docs/perf/samples/`.

## What this is not

- Not browser RUM, not Core Web Vitals from real users.
- Not multi-region API p95.
- Not a substitute for post-deploy measurement on a target machine.

## Success criteria for production claims

Only after a real package is installed on a representative Windows host with real data volume:

- Collect app startup waterfall (binary cold start → first UI interactive).
- Collect authenticated API p95 for account page / checkin status / dashboard ops.
- Store samples with machine class, data size, and commit SHA.

## Local sample schema

```json
{
  "generatedAt": "ISO-8601",
  "commit": "git sha or unknown",
  "host": "COMPUTERNAME",
  "kind": "local-loopback",
  "health": { "ok": true, "latencyMs": 12, "url": "http://127.0.0.1:PORT/api/health" },
  "commands": [{ "name": "npm test", "ok": true, "durationMs": 1500 }]
}
```
