# Task Plan: Operator Acceptance Record

## Goal

Turn the final human launch evidence into a packaged, repeatable record template so the operator can capture production approval, target-machine acceptance, first-hour monitoring, and rollback readiness without inventing a format.

This slice should not change product behavior. It only changes release documentation and package contents.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Documentation - complete
- Phase 3: Packaging verification - complete
- Phase 4: Commit - complete

## Task List

- [x] OBSERVE: Review current launch readiness docs, operator runbook, package script, and latest package manifest.
- [x] DOCS: Add a no-secrets operator acceptance record template.
- [x] DOCS: Link the template from the operator runbook and launch readiness notes.
- [x] SCRIPT: Include the template in package manifest/checksums/zip contents.
- [x] CHECK: Run parser, diff, package, manifest/content, and package-local acceptance checks.
- [x] COMMIT: Commit and push this release-handoff improvement.

## Verification Target

- `rtk powershell -NoProfile -Command '$errors = $null; [System.Management.Automation.PSParser]::Tokenize((Get-Content -Raw "scripts\package-release.ps1"), [ref]$errors) | Out-Null; if ($errors.Count -gt 0) { $errors | Format-List *; exit 1 }'`
- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1`
- Latest package contains `docs\OPERATOR_ACCEPTANCE_RECORD.md`.
- Package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30`
