# Findings: Final Launch Audit

## Source Findings

- Current branch is `main...origin/main` and clean before this slice.
- Latest release packaging commits are `679ff78 Add release package manifest script` and `210a140 Close release package manifest plan`.
- Existing launch docs now include the release gate, operator runbook, package script, rollback plan, first-hour monitoring, and package checksum handoff.

## Review Findings

- Fixed a launch-doc consistency issue: `docs\LAUNCH_READINESS.md` still instructed the operator to start `dist\relaycheck.exe` directly even though the current handoff path is the `scripts\package-release.ps1` zip package.
- PowerShell parser checks passed for `scripts\verify-release.ps1`, `scripts\package-release.ps1`, and `scripts\operator-acceptance.ps1`.
- `rtk git diff --check` passed.
- No UTF-8 BOM was found in scanned source, docs, or release scripts.
- Mojibake probe for the previously seen Chinese corruption patterns found no source/docs/script matches.
- Generated/runtime paths checked with Git pathspecs: `data`, `dist`, `.tmp`, `frontend/dist`, and `frontend/node_modules` are not tracked.
- Production-debug marker scan found no release blocker. Hits were limited to smoke-script `console.log` usage, test fixture strings, CSS token names, and documentation.
- Focused secret-pattern scan found no real credential leak evidence. Hits were test fixture values such as `sk-plain-api-key-for-test` and task/CSS identifiers.

## Verification Findings

- Full release gate passed on 2026-07-05 with `-ProxyUrl http://127.0.0.1:7897`.
- Frontend unit tests passed: 14 files / 216 tests.
- Go tests passed across 12 packages; `go vet` passed.
- Frontend production build and Windows `dist\relaycheck.exe` build passed.
- `npm audit --audit-level=low` reported 0 vulnerabilities.
- `govulncheck` reported current code affected by 0 vulnerabilities; it also reported 1 imported package vulnerability that current code does not call.
- Binary health smoke passed: temporary `relaycheck.exe` returned `/api/health` 200 and fresh DB `/api/channels` shape was OK.
- Browser smoke passed: scheduler layout smoke passed at 1440x900 and 390x900; navigation intent smoke passed 9/9.

## Package Findings

- Clean-tree package was created from commit `05dcc933877bbdef7f193739d87ce66746c9c2a2`.
- Package directory: `dist\releases\relaycheck-desktop-1.1.0-05dcc933877b-20260705-141058`.
- Package zip: `dist\releases\relaycheck-desktop-1.1.0-05dcc933877b-20260705-141058.zip`.
- Zip SHA256 sidecar matched an independent `Get-FileHash` calculation: `9777db21fc5f4387cd7fd2819d515a068595a4e245d428ec9e8cd0695a52d4e1`.
- `manifest.json` reported `version=v1.1.0`, `gitDirty=false`, and commit `05dcc933877bbdef7f193739d87ce66746c9c2a2`.
- Package contents were present: `relaycheck.exe`, `README.md`, `docs\LAUNCH_READINESS.md`, `docs\OPERATOR_RUNBOOK.md`, `scripts\operator-acceptance.ps1`, `manifest.json`, and `checksums.sha256`.
- Package-local operator acceptance passed on port 3102. The only warnings were expected relative `data\relaycheck.db` and `data\backups` path reminders for the operator working directory.

## Final Review Verdict

- No release-blocking findings remain in this local pre-launch audit.
- Production rollout still requires the human operator to approve the package, set any required bootstrap environment, back up existing data, run the package on the target machine, and record first real operator-run evidence.
