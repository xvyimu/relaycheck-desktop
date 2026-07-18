[CmdletBinding()]
param(
  [string]$ExePath = "",
  [string]$BaseUrl = "http://127.0.0.1:3001",
  [int]$TimeoutSec = 60,
  [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($ExePath)) {
  $ExePath = Join-Path $RepoRoot "dist\relaycheck.exe"
}
if (-not (Test-Path -LiteralPath $ExePath)) {
  throw "Binary not found: $ExePath (build with go build -mod=vendor -o dist\relaycheck.exe .)"
}
if ([string]::IsNullOrWhiteSpace($OutDir)) {
  $OutDir = Join-Path $RepoRoot "docs\perf\samples"
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

# Ensure no existing instance holds the port/data dir.
Get-Process -Name "relaycheck" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400

$healthUrl = ($BaseUrl.TrimEnd('/') + "/api/health")
$sha = "unknown"
try {
  Push-Location $RepoRoot
  $gitSha = (git rev-parse --short HEAD 2>$null)
  if ($LASTEXITCODE -eq 0 -and $gitSha) { $sha = "$gitSha".Trim() }
} finally {
  Pop-Location
}

$logOut = Join-Path $OutDir "cold-start-stdout.log"
$logErr = Join-Path $OutDir "cold-start-stderr.log"
Remove-Item -LiteralPath $logOut, $logErr -Force -ErrorAction SilentlyContinue

$swTotal = [Diagnostics.Stopwatch]::StartNew()
$proc = Start-Process -FilePath $ExePath -WorkingDirectory $RepoRoot -PassThru -WindowStyle Hidden `
  -RedirectStandardOutput $logOut -RedirectStandardError $logErr
$spawnMs = [int]$swTotal.Elapsed.TotalMilliseconds

$firstHealthMs = $null
$status = $null
$deadline = (Get-Date).AddSeconds($TimeoutSec)
while ((Get-Date) -lt $deadline) {
  try {
    $resp = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -TimeoutSec 2
    if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
      $firstHealthMs = [int]$swTotal.Elapsed.TotalMilliseconds
      $status = [int]$resp.StatusCode
      break
    }
  } catch {
    Start-Sleep -Milliseconds 100
  }
}
$swTotal.Stop()

if ($null -eq $firstHealthMs) {
  if ($proc -and -not $proc.HasExited) { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue }
  throw "Timed out waiting for $healthUrl after ${TimeoutSec}s (spawnMs=$spawnMs)"
}

# Optional second probe: system status readiness (not UI first-interactive).
$systemMs = $null
$swSys = [Diagnostics.Stopwatch]::StartNew()
try {
  $sys = Invoke-WebRequest -Uri ($BaseUrl.TrimEnd('/') + "/api/system/status") -UseBasicParsing -TimeoutSec 10
  $swSys.Stop()
  if ($sys.StatusCode -ge 200 -and $sys.StatusCode -lt 500) {
    $systemMs = $firstHealthMs + [int]$swSys.Elapsed.TotalMilliseconds
  }
} catch {
  $swSys.Stop()
}

$report = [ordered]@{
  generatedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  commit = $sha
  kind = "local-cold-start-process-waterfall"
  note = "Process spawn → first /api/health. Does NOT measure Wails/WebView first-interactive paint."
  exe = $ExePath
  baseUrl = $BaseUrl
  pid = $proc.Id
  marksMs = [ordered]@{
    processSpawn = $spawnMs
    firstHealth = $firstHealthMs
    systemStatusAfterHealth = $systemMs
  }
  healthStatus = $status
  host = [ordered]@{
    computerName = $env:COMPUTERNAME
    os = [System.Environment]::OSVersion.VersionString
    processors = [Environment]::ProcessorCount
  }
}

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$outFile = Join-Path $OutDir ("local-cold-start-" + $stamp + ".json")
($report | ConvertTo-Json -Depth 6) | Set-Content -LiteralPath $outFile -Encoding utf8

# Stop the process we started so later sessions stay clean.
if ($proc -and -not $proc.HasExited) {
  Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Milliseconds 300
Remove-Item -LiteralPath $logOut, $logErr -Force -ErrorAction SilentlyContinue

Write-Host "WROTE $outFile"
Write-Host ("spawnMs={0} firstHealthMs={1} systemStatusMs={2}" -f $spawnMs, $firstHealthMs, $systemMs)
$report | ConvertTo-Json -Depth 6
