# Progress: Release Package Manifest

## 2026-07-05

- Started release packaging continuation after `4914607 Close operator launch runbook plan`.
- Confirmed repository state was clean on `main...origin/main`.
- Reviewed `shipping-and-launch` guidance for launch artifacts, rollback, and operator monitoring.
- Reviewed `.gitignore`, README, `docs\LAUNCH_READINESS.md`, `docs\OPERATOR_RUNBOOK.md`, `main.go`, `scripts\verify-release.ps1`, and product version references.
- Chose `dist\releases` as the generated package output location because `dist/` is already ignored.
- Added `scripts\package-release.ps1` to build/package `relaycheck.exe`, README, launch docs, operator runbook, package manifest, internal checksums, zip, and zip sidecar checksum.
- Updated README, `docs\LAUNCH_READINESS.md`, and `docs\OPERATOR_RUNBOOK.md` with packaging instructions.
- Updated `scripts\operator-acceptance.ps1` so package-local `scripts\operator-acceptance.ps1 -StartReleaseExe` can find package-root `relaycheck.exe`.
- Ran PowerShell parser checks for `scripts\package-release.ps1` and `scripts\operator-acceptance.ps1`; passed.
- Ran `scripts\package-release.ps1 -SkipBuild -AllowDirty`; generated `dist\releases\relaycheck-desktop-1.1.0-49146079b7c0-20260705-122410.zip`.
- Verified `.zip.sha256` matched the actual package hash `c340ae2ccb681a558cdc51b07220416e226b54a3b982554c59e0cb55652bd9d7`.
- Inspected package contents: `relaycheck.exe`, `README.md`, `docs\LAUNCH_READINESS.md`, `docs\OPERATOR_RUNBOOK.md`, `scripts\operator-acceptance.ps1`, `manifest.json`, and `checksums.sha256`.
- Ran package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30`; passed.
- Confirmed generated package artifacts are ignored by `.gitignore` through `git check-ignore -v`.
- Checked generated zip entries for `data/`, `.db`, secret, token, and cookie naming patterns; found 0 entries.
- Ran tracked diff secret-keyword review; hits are safety documentation/placeholders only, not real secrets.
- Committed and pushed `679ff78 Add release package manifest script` to `origin/main`.
- Ran a clean-tree packaging verification after commit: `scripts\package-release.ps1 -SkipBuild` produced `relaycheck-desktop-1.1.0-679ff782f62b-20260705-135058.zip`.
- Verified the final package zip SHA256 sidecar matched `232db97f30cd4a747e7fcd1480a27a8c055b221e157b391a7fcde7886d1c2901`, manifest `gitDirty=false`, and package-local acceptance passed on port 3102.
