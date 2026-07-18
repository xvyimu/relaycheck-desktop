[CmdletBinding()]
param(
  [string]$HealthUrl = "http://127.0.0.1:3001/api/health",
  [switch]$RunFrontendGates,
  [string]$OutputDir = ""
)

$ErrorActionPreference = "Continue"
Set-StrictMode -Version Latest

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $RepoRoot "docs\perf\samples"
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

function Get-GitSha {
  try {
    Push-Location $RepoRoot
    $sha = (git rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -eq 0 -and $sha) { return $sha.Trim() }
  } catch {} finally { Pop-Location }
  return "unknown"
}

function Measure-Http([string]$Url) {
  $sw = [Diagnostics.Stopwatch]::StartNew()
  try {
    $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
    $sw.Stop()
    return @{ ok = ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500); latencyMs = [int]$sw.Elapsed.TotalMilliseconds; status = [int]$resp.StatusCode; error = $null }
  } catch {
    $sw.Stop()
    return @{ ok = $false; latencyMs = [int]$sw.Elapsed.TotalMilliseconds; status = 0; error = $_.Exception.Message }
  }
}

function Measure-CommandNamed([string]$Name, [scriptblock]$Block) {
  $sw = [Diagnostics.Stopwatch]::StartNew()
  $ok = $true
  $err = $null
  try {
    & $Block | Out-Null
    if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) { $ok = $false; $err = "exit $LASTEXITCODE" }
  } catch {
    $ok = $false
    $err = $_.Exception.Message
  }
  $sw.Stop()
  return @{ name = $Name; ok = $ok; durationMs = [int]$sw.Elapsed.TotalMilliseconds; error = $err }
}

$health = Measure-Http $HealthUrl
$commands = @()
if ($RunFrontendGates) {
  $fe = Join-Path $RepoRoot "frontend"
  $commands += Measure-CommandNamed "npm test" { Set-Location $fe; npm test }
  $commands += Measure-CommandNamed "npm run build" { Set-Location $fe; npm run build }
}

$sample = [ordered]@{
  generatedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  commit      = Get-GitSha
  host        = $env:COMPUTERNAME
  kind        = "local-loopback"
  note        = "Not production RUM. See docs/perf/README.md."
  health      = @{
    url       = $HealthUrl
    ok        = [bool]$health.ok
    latencyMs = [int]$health.latencyMs
    status    = [int]$health.status
    error     = $health.error
  }
  commands    = $commands
}

$stamp = (Get-Date).ToString("yyyyMMdd-HHmmss")
$outFile = Join-Path $OutputDir ("local-sample-" + $stamp + ".json")
($sample | ConvertTo-Json -Depth 6) | Set-Content -LiteralPath $outFile -Encoding utf8
Write-Host "Wrote $outFile"
Write-Host ("health.ok=" + $sample.health.ok + " latencyMs=" + $sample.health.latencyMs)
if (-not $sample.health.ok) {
  Write-Host "Health probe failed (instance may be stopped). Sample still written for audit."
}
exit 0
