# Task Plan: Operator Launch Helper

## Goal

Add a package-local operator launch helper that validates the package, starts `relaycheck.exe`, waits for health, runs operator acceptance, and writes a no-secrets automated launch record.

This slice should not change product behavior. It only adds a release/operator script, package contents, and operator documentation.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Implementation - complete
- Phase 3: Verification - complete
- Phase 4: Commit - complete

## Task List

- [x] OBSERVE: Review current package verifier, operator acceptance script, package script, and launch docs.
- [x] SCRIPT: Add `scripts\operator-launch.ps1`.
- [x] SCRIPT: Include the launch helper in package contents, manifest, and verifier requirements.
- [x] DOCS: Document package-local operator launch flow and no-secrets launch record output.
- [x] CHECK: Run parser checks, development package checks, clean package checks, and package-local launch helper smoke.
- [x] COMMIT: Commit and push this release-handoff improvement.

## Verification Target

- Parser checks for `scripts\operator-launch.ps1`, `scripts\verify-package.ps1`, and `scripts\package-release.ps1`.
- Development package smoke with `-AllowDirty`.
- Package-local `scripts\operator-launch.ps1 -Port 3102 -TimeoutSeconds 30 -NoOpen -StopAfterAcceptance`.
- Clean package smoke after commit.
