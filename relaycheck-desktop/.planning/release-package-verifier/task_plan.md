# Task Plan: Release Package Verifier

## Goal

Add a repeatable package-integrity verifier so release handoff does not depend on manual zip, manifest, and checksum inspection.

This slice should not change product behavior. It only adds a release script, package contents, and operator documentation.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Implementation - complete
- Phase 3: Verification - in_progress
- Phase 4: Commit - pending

## Task List

- [x] OBSERVE: Review current release package, package script, operator runbook, and launch readiness docs.
- [x] SCRIPT: Add `scripts\verify-package.ps1`.
- [x] SCRIPT: Include the verifier in release package contents, manifest, and checksums.
- [x] DOCS: Document source-tree zip verification and package-local verification.
- [ ] CHECK: Run parser checks, verifier checks, package build, clean package verification, and package-local acceptance.
- [ ] COMMIT: Commit and push this release-handoff improvement.

## Verification Target

- Parser checks for `scripts\verify-package.ps1` and `scripts\package-release.ps1`.
- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1`
- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\verify-package.ps1`
- Package-local `scripts\verify-package.ps1 -PackageDir .`
- Package-local `scripts\operator-acceptance.ps1 -StartReleaseExe -ExpectedPort 3102 -TimeoutSeconds 30`
