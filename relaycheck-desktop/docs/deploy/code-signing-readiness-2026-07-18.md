# Code signing readiness (Authenticode)

- **Date:** 2026-07-18
- **Status:** **Blocked on materials** — tooling ready; **no Authenticode code-signing cert on this machine** (2026-07-18 exhaustive recheck).

## Exhaustive local search (2026-07-18)

| Probe | Result |
|---|---|
| `RELAYCHECK_SIGN_PFX` / `…_PASSWORD` env | unset |
| `signtool.exe` | present (Windows Kits 10.0.26100) |
| `dist\relaycheck.exe` | present after local build; **unsigned** |
| Filesystem `*.pfx` under user/Documents/Downloads/Desktop + D:/E: secrets|certs | **0 hits** |
| `Cert:\CurrentUser\My` private keys | Phone helper + ASP.NET Core **localhost HTTPS** only — **no Code Signing EKU** |
| `Cert:\LocalMachine\My` | no usable code-signing private key |

**Explicit non-action:** do **not** mint a self-signed “production” cert for distribution (SmartScreen noise; policy in this doc).

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
