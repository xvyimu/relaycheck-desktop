# Task Plan: Operator Monitor

## Goal

Add a package-local first-hour monitor helper that samples RelayCheck read-only health and scheduler endpoints, records no-secrets evidence, and turns launch monitoring into a repeatable operator command.

This slice should not change product runtime behavior. It only adds release/operator tooling, package contents, and operator documentation.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Implementation - complete
- Phase 3: Verification - complete
- Phase 4: Commit - in progress

## Task List

- [x] OBSERVE: Review package verifier, package builder, launch helper, acceptance script, and launch docs.
- [x] SCRIPT: Add scripts/operator-monitor.ps1.
- [x] SCRIPT: Include the monitor helper in package contents, manifest, and verifier requirements.
- [x] DOCS: Document first-hour monitor usage and generated monitor records.
- [x] CHECK: Run parser checks, development package checks, package-local launch plus monitor smoke, and clean package checks.
- [ ] COMMIT: Commit and push the release-monitoring improvement.

## Verification Target

- Parser checks for scripts/operator-monitor.ps1, scripts/verify-package.ps1, and scripts/package-release.ps1.
- rtk git diff --check.
- Development package with -AllowDirty.
- Package-local scripts/verify-package.ps1 -PackageDir . -AllowDirtyManifest.
- Package-local launch smoke with an isolated runtime.
- Package-local monitor smoke with -SampleCount 3 -IntervalSeconds 1.
- Clean package and package-local monitor smoke after commit.
