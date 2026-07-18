# Code signing readiness (Authenticode)

- **Date:** 2026-07-18
- **Status:** **Blocked on materials** — `signtool.exe` + `scripts/sign-release.ps1` ready; `RELAYCHECK_SIGN_PFX` / password still unset (2026-07-18 recheck). Binary may exist at `dist\relaycheck.exe` after local build but remains **unsigned**.

## Goal

Optional Windows Authenticode signature for `dist\relaycheck.exe` / release zip payloads so SmartScreen trust improves.

## Required inputs (operator provides outside git)

| Input | Example env | Notes |
|---|---|---|
| Code signing certificate PFX | `RELAYCHECK_SIGN_PFX` absolute path | **Never commit PFX** |
| PFX password | `RELAYCHECK_SIGN_PFX_PASSWORD` | Secret store / CI secret |
| Timestamp URL | `RELAYCHECK_SIGN_TIMESTAMP_URL` | e.g. DigiCert/Sectigo RFC3161 |
| Optional description | `RELAYCHECK_SIGN_DESC` | File description string |

## Local dry-run without cert

```powershell
# Verifies tool discovery only; exits non-zero if cert missing (expected).
powershell -NoProfile -File .\scripts\sign-release.ps1 -WhatIf
```

## When cert is available

```powershell
$env:RELAYCHECK_SIGN_PFX = "D:\secrets\codesign.pfx"
$env:RELAYCHECK_SIGN_PFX_PASSWORD = "***"
powershell -NoProfile -File .\scripts\package-release.ps1
powershell -NoProfile -File .\scripts\sign-release.ps1 -Path .\dist\relaycheck.exe
Get-AuthenticodeSignature .\dist\relaycheck.exe
```

## Explicit non-goals without materials

- Do not generate self-signed production certs for distribution.
- Do not embed passwords in scripts or repo files.
- MSIX / Store listing is out of scope for this loopback desktop product unless separately requested.
