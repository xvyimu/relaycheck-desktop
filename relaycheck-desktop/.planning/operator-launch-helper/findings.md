# Findings: Operator Launch Helper

## Source Findings

- `scripts\verify-package.ps1` can now verify package integrity both from the source tree and from an extracted package root.
- `scripts\operator-acceptance.ps1` can validate a running app, and it can also start/stop a temporary fresh runtime for smoke testing.
- The remaining first-launch path still requires the operator to manually sequence package verification, app startup, health wait, acceptance, and record capture.

## Implementation Findings

- Added `scripts\operator-launch.ps1`.
- The launch helper validates the package, starts `relaycheck.exe`, waits for health, runs operator acceptance, and writes a no-secrets record under `launch-records\`.
- The launch helper captures child-process stdout/stderr into `launch-records\relaycheck-stdout.log` and `launch-records\relaycheck-stderr.log`.
- Added `-RuntimeDir` and `-StopAfterAcceptance` support so release verification can run an isolated fresh-runtime smoke without using the package root `data\` directory.
- The helper detects RelayCheck port fallback candidates and reports an explicit port-conflict failure unless `-AllowPortConflict` is set.
- Updated package contents, manifest, package verifier requirements, launch docs, and acceptance-record fields for the launch helper.

## Verification Findings

- Parser checks passed for `scripts\operator-launch.ps1`, `scripts\verify-package.ps1`, and `scripts\package-release.ps1`.
- `rtk git diff --check` passed.
- Development packaging with `-AllowDirty` passed and included the launch helper in package contents.
- Package-local `scripts\verify-package.ps1 -PackageDir . -AllowDirtyManifest` passed.
- Initial package-root launch smoke exposed an expected port-fallback handling gap; the helper was updated to detect fallback ports and to capture child-process logs.
- Isolated package-local launch smoke passed with `scripts\operator-launch.ps1 -Port 3207 -TimeoutSeconds 45 -NoOpen -StopAfterAcceptance -AllowDirtyManifest -RuntimeDir .tmp\operator-launch-runtime`.
- Clean-tree packaging passed after commit `33376b7d8a88692274463e4d79b87d53f90f16f6`.
- Clean package: `dist\releases\relaycheck-desktop-1.1.0-33376b7d8a88-20260705-152040.zip`.
- Clean package SHA256: `96d89d0eb82f2921353038e3baaebb9d6aa0ea98d99da30354fc8090981bb67a`.
- Clean source-tree package verification passed with `manifest.gitDirty=false`.
- Clean package-local verification passed with `manifest.gitDirty=false`.
- Clean package-local operator launch smoke passed on port 3207 and generated a no-secrets launch record.
