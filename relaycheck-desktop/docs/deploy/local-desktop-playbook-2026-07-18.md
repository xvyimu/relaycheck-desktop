# Local desktop deploy / package playbook

- **Date:** 2026-07-18
- **Product:** RelayCheck Desktop (loopback single-user binary + embedded UI)
- **Meaning of "deploy" here:** build a release package on this machine and verify it. **Not** cloud/SaaS deploy.

## Preconditions

- Clean or intentionally dirty tree (`package-release.ps1 -AllowDirty` if needed).
- Go release toolchain per `.go-version` / scripts (1.26.5 for package).
- Node/npm per `frontend/package.json` engines when packaging.
- **Do not** point scripts at production DBs; use repo `data/` only for local runs.

## Recommended local release path

```powershell
cd E:\zidqiandao\relaycheck-desktop

# 1) Full verifier (tests + build + optional browser smoke)
powershell -NoProfile -File .\scripts\verify-release.ps1 -BrowserPort 5174

# 2) Package (exact toolchain enforced inside script)
powershell -NoProfile -File .\scripts\package-release.ps1 -AllowDirty

# 3) Verify package zip/manifest if produced
powershell -NoProfile -File .\scripts\verify-package.ps1
```

## Outputs (typical)

- `dist\relaycheck.exe` — local binary
- `dist\releases\*.zip` — packaged release (when package-release succeeds)
- Never commit `dist/` (gitignored)

## Signing / store distribution

**Out of scope without materials:** Authenticode cert, SmartScreen, MSIX store listing.  
If signing is required later: document cert location outside repo, never commit PFX.

## Production RUM

Use `docs/perf/README.md` + `scripts/sample-local-perf.ps1` for **local** samples only.  
Production p95 requires a representative host + real data volume after install.
