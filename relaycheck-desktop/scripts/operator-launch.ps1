[CmdletBinding()]
param(
  [string]$PackageDir = "",
  [int]$Port = 3001,
  [int]$TimeoutSeconds = 30,
  [switch]$NoOpen,
  [switch]$OpenBrowser,
  [switch]$SkipPackageVerification,
  [switch]$SkipAcceptance,
  [switch]$AllowDirtyManifest,
  [switch]$AllowDegradedHealth,
  [switch]$AllowPortConflict,
  [switch]$StopAfterAcceptance,
  [string]$RuntimeDir = "",
  [string]$RecordPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

function Write-Step {
  param([string]$Message)
  Write-Host ""
  Write-Host "==> $Message"
}

function Write-Pass {
  param([string]$Message)
  Write-Host "[PASS] $Message"
}

function Write-Warn {
  param([string]$Message)
  Write-Host "[WARN] $Message"
}

function Get-PropertyValue {
  param(
    [object]$Object,
    [string]$Name
  )
  if ($null -eq $Object) {
    return $null
  }
  $property = $Object.PSObject.Properties[$Name]
  if ($null -eq $property) {
    return $null
  }
  return $property.Value
}

function Join-ApiUrl {
  param(
    [string]$BaseUrl,
    [string]$Path
  )
  return $BaseUrl.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-ApiPayload {
  param(
    [string]$BaseUrl,
    [string]$Path
  )

  $url = Join-ApiUrl $BaseUrl $Path
  $response = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec $TimeoutSeconds
  if ($response.StatusCode -ne 200) {
    throw "GET $url returned HTTP $($response.StatusCode)"
  }
  $payload = $response.Content | ConvertFrom-Json
  if ((Get-PropertyValue $payload "ok") -ne $true) {
    throw "GET $url returned ok=false"
  }
  return $payload
}

function Wait-ApiReady {
  param(
    [int]$PreferredPort,
    [System.Diagnostics.Process]$Process
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    if ($Process -and $Process.HasExited) {
      throw "relaycheck.exe exited before health was ready with exit code $($Process.ExitCode)"
    }
    foreach ($candidatePort in (Get-PortCandidates $PreferredPort)) {
      $candidateBaseUrl = "http://127.0.0.1:$candidatePort"
      try {
        $payload = Invoke-ApiPayload $candidateBaseUrl "/api/health"
        return @{
          payload = $payload
          baseUrl = $candidateBaseUrl
          port = $candidatePort
        }
      } catch {
        continue
      }
    }
    Start-Sleep -Milliseconds 500
  } while ((Get-Date) -lt $deadline)

  throw "Timed out waiting for RelayCheck health on preferred port $PreferredPort or fallback ports"
}

function Get-PortCandidates {
  param([int]$PreferredPort)

  $candidates = New-Object System.Collections.Generic.List[int]
  foreach ($candidate in @($PreferredPort, 3001, 3010, 3000, 3002, 3003, 8080, 9999, 7897)) {
    if ($candidate -gt 0 -and -not $candidates.Contains($candidate)) {
      $candidates.Add($candidate)
    }
  }
  for ($candidate = 3011; $candidate -lt 3030; $candidate++) {
    if (-not $candidates.Contains($candidate)) {
      $candidates.Add($candidate)
    }
  }
  return $candidates.ToArray()
}

function Assert-UnderPackage {
  param(
    [string]$PackageRoot,
    [string]$Path,
    [string]$Label
  )

  $rootFull = [System.IO.Path]::GetFullPath($PackageRoot).TrimEnd("\", "/")
  $rootWithSeparator = $rootFull + [System.IO.Path]::DirectorySeparatorChar
  $full = [System.IO.Path]::GetFullPath($Path)
  if ($full -ne $rootFull -and -not $full.StartsWith($rootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "$Label is outside package root: $full"
  }
  return $full
}

function Get-Manifest {
  param([string]$PackageRoot)

  $manifestPath = Join-Path $PackageRoot "manifest.json"
  if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    throw "manifest.json not found: $manifestPath"
  }
  return Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
}

function Write-LaunchRecord {
  param(
    [string]$Path,
    [hashtable]$Record
  )

  $lines = @(
    "# Operator Launch Record",
    "",
    "Generated at UTC: $($Record.generatedAtUtc)",
    "Package root: $($Record.packageRoot)",
    "Runtime dir: $($Record.runtimeRoot)",
    "Base URL: $($Record.baseUrl)",
    "Requested port: $($Record.port)",
    "Actual port: $($Record.actualPort)",
    "",
    "## Package",
    "",
    "| Field | Value |",
    "| --- | --- |",
    "| Version | $($Record.version) |",
    "| Git commit | $($Record.gitCommit) |",
    "| Git dirty | $($Record.gitDirty) |",
    "| Package verification | $($Record.packageVerification) |",
    "",
    "## Launch",
    "",
    "| Field | Value |",
    "| --- | --- |",
    "| Process ID | $($Record.processId) |",
    "| Health status | $($Record.healthStatus) |",
    "| Operator acceptance | $($Record.operatorAcceptance) |",
    "| Stop after acceptance | $($Record.stopAfterAcceptance) |",
    "| Stdout log | $($Record.stdoutPath) |",
    "| Stderr log | $($Record.stderrPath) |",
    "| Automated result | $($Record.result) |",
    "",
    "## Notes",
    "",
    "- This record is generated by `scripts\\operator-launch.ps1`.",
    "- Do not add passwords, cookies, bearer tokens, API keys, exported `.rczip` passwords, or private database contents.",
    "- Complete `docs\\OPERATOR_ACCEPTANCE_RECORD.md` for the human final launch decision."
  )

  if (-not [string]::IsNullOrWhiteSpace($Record.error)) {
    $lines += @("", "## Error", "", $Record.error)
  }

  $recordDir = Split-Path -Parent $Path
  New-Item -ItemType Directory -Force $recordDir | Out-Null
  Set-Content -LiteralPath $Path -Value $lines -Encoding UTF8
}

if ($Port -le 0) {
  throw "-Port must be a positive port."
}
if ($NoOpen -and $OpenBrowser) {
  throw "Use either -NoOpen or -OpenBrowser, not both."
}

if ([string]::IsNullOrWhiteSpace($PackageDir)) {
  $PackageDir = Join-Path $PSScriptRoot ".."
}
$packageRoot = (Resolve-Path -LiteralPath $PackageDir).Path
$releaseExe = Join-Path $packageRoot "relaycheck.exe"
$verifyScript = Join-Path $packageRoot "scripts\verify-package.ps1"
$acceptanceScript = Join-Path $packageRoot "scripts\operator-acceptance.ps1"
$baseUrl = "http://127.0.0.1:$Port"
$startedProcess = $null
$oldNoOpen = $env:RELAYCHECK_NO_OPEN
$oldPort = $env:RELAYCHECK_PORT
$record = @{
  generatedAtUtc = (Get-Date).ToUniversalTime().ToString("o")
  packageRoot = $packageRoot
  runtimeRoot = ""
  baseUrl = $baseUrl
  port = $Port
  actualPort = ""
  version = ""
  gitCommit = ""
  gitDirty = ""
  packageVerification = "not_run"
  processId = ""
  healthStatus = "not_run"
  operatorAcceptance = "not_run"
  stopAfterAcceptance = [string][bool]$StopAfterAcceptance
  stdoutPath = ""
  stderrPath = ""
  result = "fail"
  error = ""
}

if ([string]::IsNullOrWhiteSpace($RecordPath)) {
  $stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss")
  $RecordPath = Join-Path $packageRoot "launch-records\operator-launch-$stamp.md"
}
$recordFull = Assert-UnderPackage $packageRoot $RecordPath "record path"
$recordDir = Split-Path -Parent $recordFull
if ([string]::IsNullOrWhiteSpace($RuntimeDir)) {
  $runtimeRoot = $packageRoot
} else {
  $runtimeRoot = Assert-UnderPackage $packageRoot $RuntimeDir "runtime directory"
  New-Item -ItemType Directory -Force $runtimeRoot | Out-Null
}
$stdoutPath = Join-Path $recordDir "relaycheck-stdout.log"
$stderrPath = Join-Path $recordDir "relaycheck-stderr.log"
$record.runtimeRoot = $runtimeRoot
$record.stdoutPath = $stdoutPath
$record.stderrPath = $stderrPath

try {
  Write-Host "RelayCheck operator launch"
  Write-Host "Package root: $packageRoot"
  Write-Host "Runtime dir: $runtimeRoot"
  Write-Host "BaseUrl: $baseUrl"

  Write-Step "Manifest"
  $manifest = Get-Manifest $packageRoot
  $record.version = [string](Get-PropertyValue $manifest "version")
  $record.gitCommit = [string](Get-PropertyValue $manifest "gitCommit")
  $record.gitDirty = [string][bool](Get-PropertyValue $manifest "gitDirty")
  Write-Pass "manifest loaded: $($record.version) $($record.gitCommit)"

  if (-not $SkipPackageVerification) {
    Write-Step "Package verification"
    if (-not (Test-Path -LiteralPath $verifyScript -PathType Leaf)) {
      throw "Package verifier not found: $verifyScript"
    }
    $verifyArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $verifyScript, "-PackageDir", $packageRoot)
    if ($AllowDirtyManifest) {
      $verifyArgs += "-AllowDirtyManifest"
    }
    & powershell @verifyArgs
    if ($LASTEXITCODE -ne 0) {
      throw "Package verification failed with exit code $LASTEXITCODE"
    }
    $record.packageVerification = "pass"
  } else {
    Write-Warn "package verification skipped"
    $record.packageVerification = "skipped"
  }

  Write-Step "Start relaycheck.exe"
  if (-not (Test-Path -LiteralPath $releaseExe -PathType Leaf)) {
    throw "Release executable not found: $releaseExe"
  }
  $env:RELAYCHECK_PORT = [string]$Port
  if ($OpenBrowser) {
    $env:RELAYCHECK_NO_OPEN = $null
  } else {
    $env:RELAYCHECK_NO_OPEN = "1"
  }
  New-Item -ItemType Directory -Force $recordDir | Out-Null
  $startedProcess = Start-Process -FilePath $releaseExe -WorkingDirectory $runtimeRoot -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru
  $record.processId = [string]$startedProcess.Id
  Write-Pass "relaycheck.exe started with pid=$($startedProcess.Id)"

  Write-Step "Wait for health"
  $healthReady = Wait-ApiReady $Port $startedProcess
  $baseUrl = [string]$healthReady.baseUrl
  $actualPort = [int]$healthReady.port
  $record.baseUrl = $baseUrl
  $record.actualPort = [string]$actualPort
  if ($actualPort -ne $Port) {
    if ($AllowPortConflict) {
      Write-Warn "preferred port $Port fell back to $actualPort and is explicitly allowed"
    } else {
      throw "preferred port $Port fell back to $actualPort; pass -AllowPortConflict to accept this launch"
    }
  }
  $healthPayload = $healthReady.payload
  $health = Get-PropertyValue $healthPayload "data"
  $record.healthStatus = [string](Get-PropertyValue $health "status")
  if ($record.healthStatus -eq "ok") {
    Write-Pass "/api/health status is ok"
  } elseif ($record.healthStatus -eq "degraded" -and $AllowDegradedHealth) {
    Write-Warn "/api/health status is degraded and explicitly allowed"
  } else {
    throw "/api/health status is $($record.healthStatus)"
  }

  if (-not $SkipAcceptance) {
    Write-Step "Operator acceptance"
    if (-not (Test-Path -LiteralPath $acceptanceScript -PathType Leaf)) {
      throw "Operator acceptance script not found: $acceptanceScript"
    }
    $acceptanceArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $acceptanceScript, "-BaseUrl", $baseUrl, "-ExpectedPort", [string]$actualPort, "-TimeoutSeconds", [string]$TimeoutSeconds)
    if ($AllowDegradedHealth) {
      $acceptanceArgs += "-AllowDegradedHealth"
    }
    if ($AllowPortConflict) {
      $acceptanceArgs += "-AllowPortConflict"
    }
    & powershell @acceptanceArgs
    if ($LASTEXITCODE -ne 0) {
      throw "Operator acceptance failed with exit code $LASTEXITCODE"
    }
    $record.operatorAcceptance = "pass"
  } else {
    Write-Warn "operator acceptance skipped"
    $record.operatorAcceptance = "skipped"
  }

  $record.result = "pass"
  Write-Host ""
  Write-Host "Operator launch checks passed."
} catch {
  $record.error = $_.Exception.Message
  throw
} finally {
  if ($StopAfterAcceptance -and $startedProcess -and -not $startedProcess.HasExited) {
    Stop-Process -Id $startedProcess.Id -Force -ErrorAction SilentlyContinue
  }
  $env:RELAYCHECK_NO_OPEN = $oldNoOpen
  $env:RELAYCHECK_PORT = $oldPort
  Write-LaunchRecord $recordFull $record
  Write-Host "Launch record: $recordFull"
}
