[CmdletBinding()]
param(
  [string]$DbPath = "",
  [switch]$Json
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($DbPath)) {
  $DbPath = Join-Path $RepoRoot "data\relaycheck.db"
}

if (-not (Test-Path -LiteralPath $DbPath)) {
  throw "Database not found: $DbPath (never create/delete data/ from this script)."
}

$sqlPath = Join-Path $PSScriptRoot "sql\precheck-site-orphans.sql"
if (-not (Test-Path -LiteralPath $sqlPath)) {
  throw "Missing SQL file: $sqlPath"
}

function Find-Sqlite3 {
  $cmd = Get-Command sqlite3 -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  foreach ($candidate in @(
      "C:\Program Files\SQLite\sqlite3.exe",
      "C:\sqlite\sqlite3.exe",
      "$env:LOCALAPPDATA\Programs\sqlite\sqlite3.exe"
    )) {
    if (Test-Path -LiteralPath $candidate) { return $candidate }
  }
  return $null
}

$sqlite3 = Find-Sqlite3
if (-not $sqlite3) {
  throw "sqlite3 CLI not found on PATH. Install SQLite tools or run the SQL file manually: $sqlPath"
}

# Strip .mode/.headers for machine parsing when -Json; keep human output otherwise.
$sql = Get-Content -LiteralPath $sqlPath -Raw -Encoding UTF8
if ($Json) {
  $sql = ($sql -split "`n" | Where-Object { $_ -notmatch '^\s*\.(mode|headers)\b' }) -join "`n"
  $sql = ".mode json`n" + $sql
}

$raw = & $sqlite3 -readonly $DbPath $sql 2>&1
if ($LASTEXITCODE -ne 0) {
  throw "sqlite3 failed (exit $LASTEXITCODE): $raw"
}

if ($Json) {
  # sqlite3 -mode json may emit one JSON array for the UNION.
  Write-Output $raw
  exit 0
}

$sha = "unknown"
try {
  Push-Location $RepoRoot
  $gitSha = (git rev-parse --short HEAD 2>$null)
  if ($LASTEXITCODE -eq 0 -and $gitSha) { $sha = "$gitSha".Trim() }
} catch {
} finally {
  Pop-Location
}

Write-Host "RelayCheck site orphan precheck (read-only)"
Write-Host "DB: $DbPath"
Write-Host "SHA: $sha"
Write-Host ""
Write-Output $raw
Write-Host ""
Write-Host "Interpretation:"
Write-Host "  orphan_rows = 0  -> no historical dangling child rows for that table"
Write-Host "  orphan_rows > 0  -> old deletes left orphans; cleanup needs backup + explicit confirm"
Write-Host "This script never DELETEs. See docs/sop/relaycheck-site-orphan-precheck-2026-07-18.md"
