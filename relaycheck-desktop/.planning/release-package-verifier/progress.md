# Progress: Release Package Verifier

## 2026-07-05

- Started release package verifier slice after the operator acceptance record template was packaged and pushed.
- Reviewed `scripts\package-release.ps1`, `docs\OPERATOR_RUNBOOK.md`, `docs\LAUNCH_READINESS.md`, and the active release package layout.
- Added `scripts\verify-package.ps1`.
- Updated `scripts\package-release.ps1` so the verifier is copied into the package, included in checksums, and referenced from `manifest.json`.
- Updated launch/operator docs and the acceptance-record template to require package verifier evidence.
- Verification passed so far: parser checks, `rtk git diff --check`, development packaging with `-AllowDirty`, source-tree package verification, package-local package verification, and package-local operator acceptance.
