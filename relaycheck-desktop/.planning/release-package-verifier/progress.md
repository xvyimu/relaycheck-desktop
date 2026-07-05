# Progress: Release Package Verifier

## 2026-07-05

- Started release package verifier slice after the operator acceptance record template was packaged and pushed.
- Reviewed `scripts\package-release.ps1`, `docs\OPERATOR_RUNBOOK.md`, `docs\LAUNCH_READINESS.md`, and the active release package layout.
- Added `scripts\verify-package.ps1`.
- Updated `scripts\package-release.ps1` so the verifier is copied into the package, included in checksums, and referenced from `manifest.json`.
- Updated launch/operator docs and the acceptance-record template to require package verifier evidence.
- Verification passed so far: parser checks, `rtk git diff --check`, development packaging with `-AllowDirty`, source-tree package verification, package-local package verification, and package-local operator acceptance.
- Committed the implementation as `d1e528e Add release package verifier`.
- Ran clean-tree packaging after the commit; generated `relaycheck-desktop-1.1.0-d1e528e8cbba-20260705-144314.zip` with SHA256 `5a5c8acd7fa54dc95e78a30f51f33278cc5a266f37585b69cff83d4cf1241a01` and `gitDirty=false`.
- Verified the clean package with source-tree `scripts\verify-package.ps1`, package-local `scripts\verify-package.ps1 -PackageDir .`, and package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30`.
