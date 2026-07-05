# Findings: Release Package Manifest

## Source Findings

- `scripts\verify-release.ps1` remains the authoritative release gate and already builds `dist\relaycheck.exe`.
- `docs\LAUNCH_READINESS.md` and `docs\OPERATOR_RUNBOOK.md` now cover verification and operator acceptance, but the handoff still references a loose executable rather than a package.
- `.gitignore` ignores `dist/`, `frontend/dist/`, `*.exe`, runtime `data/`, and database files, so generated package files can safely live under `dist\releases`.
- Product version is currently declared in `internal\core\routes.go` as `v1.1.0`.
- Go build currently uses `-ldflags="-H windowsgui"`; product version/build time are compile-time constants rather than ldflag-injected vars.

## Decisions

- Keep packaging as a separate script after release verification instead of folding it into the full gate.
- Package only static delivery materials: `relaycheck.exe`, launch docs, operator runbook, manifest, and SHA256 checksums.
- Require a clean Git tree by default for release packaging; allow `-AllowDirty` only for development verification.
