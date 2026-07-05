# Findings: Release Package Verifier

## Source Findings

- `scripts\package-release.ps1` writes `manifest.json`, `checksums.sha256`, a release zip, and a sibling `.zip.sha256` file.
- Operators currently have documented manual comparison steps for the zip hash and package contents.
- A package verifier can convert that manual check into an explicit pass/fail gate before copying or launching a package.

## Implementation Findings

- Added `scripts\verify-package.ps1`.
- The verifier supports source-tree zip verification with `-ZipPath` or default latest `dist\releases\*.zip`, and package-local verification with `-PackageDir .`.
- The verifier checks the zip `.sha256` sidecar, required package files, manifest product/version/commit/dirty/platform/path fields, manifest file hashes, and `checksums.sha256` entries.
- `scripts\package-release.ps1` now includes `scripts\verify-package.ps1` in package contents, `manifest.json`, and `checksums.sha256`.
- `docs\LAUNCH_READINESS.md`, `docs\OPERATOR_RUNBOOK.md`, and `docs\OPERATOR_ACCEPTANCE_RECORD.md` now include package-verifier steps/evidence.

## Verification Findings

- Parser checks passed for `scripts\verify-package.ps1` and `scripts\package-release.ps1`.
- `rtk git diff --check` passed.
- Development packaging with `-AllowDirty` passed and included the verifier in package contents.
- Source-tree package verification passed with `scripts\verify-package.ps1 -AllowDirtyManifest`.
- Package-local verification passed with `scripts\verify-package.ps1 -PackageDir . -AllowDirtyManifest`.
- Package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30` passed with only expected relative `data\` path warnings.
- Clean-tree packaging passed after commit `d1e528e8cbba1a9e0b74abaeee1d85fb617146df`.
- Clean package: `dist\releases\relaycheck-desktop-1.1.0-d1e528e8cbba-20260705-144314.zip`.
- Clean package SHA256: `5a5c8acd7fa54dc95e78a30f51f33278cc5a266f37585b69cff83d4cf1241a01`.
- Clean source-tree package verification passed with `manifest.gitDirty=false`.
- Clean package-local verification passed with `manifest.gitDirty=false`.
- Clean package-local operator acceptance passed on port 3102 with only expected relative `data\` path warnings.
