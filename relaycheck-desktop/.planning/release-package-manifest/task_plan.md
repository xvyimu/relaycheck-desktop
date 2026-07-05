# Task Plan: Release Package Manifest

## Goal

Add a repeatable local release-packaging step so RelayCheck Desktop has a concrete launch artifact beyond `dist\relaycheck.exe`: zip package, manifest, checksum file, and operator docs bundled together.

This slice should not change product behavior. Generated package artifacts stay under ignored `dist\`.

## Current Phase

- Phase 1: Planning - complete
- Phase 2: Script and documentation update - complete
- Phase 3: Verification - complete
- Phase 4: Commit - pending

## Task List

- [x] OBSERVE: Review release gate, launch docs, operator runbook, version source, build commands, and `.gitignore`.
- [x] SCRIPT: Add `scripts\package-release.ps1` to build/package the release artifact with manifest and checksums.
- [x] DOCS: Link packaging command from README, launch readiness, and operator runbook.
- [x] CHECK: Parse and run the packaging script in development mode.
- [x] CHECK: Inspect generated manifest/checksum/package contents.
- [x] REVIEW: Confirm generated artifacts are ignored and no secrets/runtime data are bundled.
- [ ] COMMIT: Commit and push the packaging slice.

## Verification Target

- `rtk powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package-release.ps1 -SkipBuild -AllowDirty`
- `rtk git diff --check`
