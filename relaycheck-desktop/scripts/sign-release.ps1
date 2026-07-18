[CmdletBinding(SupportsShouldProcess = $true)]
param(
  [string]$Path = "",
  [string]$PfxPath = $env:RELAYCHECK_SIGN_PFX,
  [string]$PfxPassword = $env:RELAYCHECK_SIGN_PFX_PASSWORD,
  [string]$TimestampUrl = $(if ($env:RELAYCHECK_SIGN_TIMESTAMP_URL) { $env:RELAYCHECK_SIGN_TIMESTAMP_URL } else { "http://timestamp.digicert.com" }),
  [string]$Description = $(if ($env:RELAYCHECK_SIGN_DESC) { $env:RELAYCHECK_SIGN_DESC } else { "RelayCheck Desktop" })
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($Path)) {
  $Path = Join-Path $RepoRoot "dist\relaycheck.exe"
}

function Find-SignTool {
  $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  $roots = @(
    "${env:ProgramFiles(x86)}\Windows Kits\10\bin",
    "${env:ProgramFiles}\Windows Kits\10\bin"
  )
  foreach ($root in $roots) {
    if (-not (Test-Path $root)) { continue }
    $hit = Get-ChildItem -Path $root -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
      Sort-Object FullName -Descending |
      Select-Object -First 1
    if ($hit) { return $hit.FullName }
  }
  return $null
}

Write-Host "Target: $Path"
$missingBinary = -not (Test-Path -LiteralPath $Path)
if ($missingBinary) {
  Write-Host "NOTE: binary not found yet: $Path"
}

$signTool = Find-SignTool
if (-not $signTool) {
  Write-Host "BLOCKED: signtool.exe not found. Install Windows SDK Signing Tools."
  exit 2
}
Write-Host "signtool: $signTool"

if ([string]::IsNullOrWhiteSpace($PfxPath) -or -not (Test-Path -LiteralPath $PfxPath)) {
  Write-Host "BLOCKED: set RELAYCHECK_SIGN_PFX to an existing PFX path outside the repo."
  Write-Host "See docs/deploy/code-signing-readiness-2026-07-18.md"
  if ($WhatIfPreference -or $missingBinary) { exit 3 }
  exit 3
}
if ([string]::IsNullOrWhiteSpace($PfxPassword)) {
  Write-Host "BLOCKED: set RELAYCHECK_SIGN_PFX_PASSWORD (do not commit secrets)."
  exit 4
}

if ($missingBinary) {
  throw "File not found: $Path (build package-release first)"
}

$args = @(
  "sign", "/fd", "SHA256",
  "/f", $PfxPath,
  "/p", $PfxPassword,
  "/tr", $TimestampUrl,
  "/td", "SHA256",
  "/d", $Description,
  $Path
)

if ($PSCmdlet.ShouldProcess($Path, "Authenticode sign")) {
  & $signTool @args
  if ($LASTEXITCODE -ne 0) { throw "signtool failed: $LASTEXITCODE" }
  Get-AuthenticodeSignature -FilePath $Path | Format-List Status, StatusMessage, SignerCertificate
  Write-Host "Signed OK: $Path"
} else {
  Write-Host "WhatIf: would run signtool sign on $Path with PFX=$PfxPath"
}
