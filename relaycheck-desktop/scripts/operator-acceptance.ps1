[CmdletBinding()]
param(
  [string]$BaseUrl = "http://127.0.0.1:3001",
  [int]$ExpectedPort = 3001,
  [int]$TimeoutSeconds = 15,
  [switch]$AllowDegradedHealth,
  [switch]$AllowPortConflict,
  [switch]$StartReleaseExe,
  [string]$ReleaseExe = "",
  [string]$RuntimeDir = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($ReleaseExe)) {
  $ReleaseExe = Join-Path $RepoRoot "dist\relaycheck.exe"
}
if ([string]::IsNullOrWhiteSpace($RuntimeDir)) {
  $RuntimeDir = Join-Path $RepoRoot ".tmp\operator-acceptance-runtime"
}

$startedProcess = $null
$oldNoOpen = $env:RELAYCHECK_NO_OPEN
$oldPort = $env:RELAYCHECK_PORT
$failures = New-Object System.Collections.Generic.List[string]

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

function Add-Failure {
  param([string]$Message)
  $failures.Add($Message)
  Write-Host "[FAIL] $Message"
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
  param([string]$Path)
  return $BaseUrl.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-ApiPayload {
  param([string]$Path)

  $url = Join-ApiUrl $Path
  try {
    $response = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec $TimeoutSeconds
  } catch {
    throw "GET $url failed: $($_.Exception.Message)"
  }

  if ($response.StatusCode -ne 200) {
    throw "GET $url returned HTTP $($response.StatusCode)"
  }

  try {
    $payload = $response.Content | ConvertFrom-Json
  } catch {
    throw "GET $url returned invalid JSON: $($_.Exception.Message)"
  }

  $ok = Get-PropertyValue $payload "ok"
  if ($ok -ne $true) {
    $errorMessage = Get-PropertyValue $payload "error"
    if ([string]::IsNullOrWhiteSpace($errorMessage)) {
      $errorMessage = "unknown API error"
    }
    throw "GET $url returned ok=false: $errorMessage"
  }

  return $payload
}

function Invoke-ApiData {
  param([string]$Path)

  $payload = Invoke-ApiPayload $Path
  $data = Get-PropertyValue $payload "data"
  Write-Output -NoEnumerate $data
}

function Assert-RepoPath {
  param([string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  if (-not $full.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to operate outside repo: $full"
  }
  return $full
}

function Remove-RepoItem {
  param([string]$Path)

  $full = Assert-RepoPath $Path
  if (Test-Path -LiteralPath $full) {
    Remove-Item -LiteralPath $full -Recurse -Force
  }
}

function Remove-RepoDirectoryIfEmpty {
  param([string]$Path)

  $full = Assert-RepoPath $Path
  if ((Test-Path -LiteralPath $full) -and -not (Get-ChildItem -LiteralPath $full -Force -ErrorAction SilentlyContinue)) {
    Remove-Item -LiteralPath $full -Force
  }
}

function Wait-ApiReady {
  param([string]$Url)

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
      if ($response.StatusCode -eq 200) {
        return
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  } while ((Get-Date) -lt $deadline)

  throw "Timed out waiting for $Url"
}

function Assert-NotBlank {
  param(
    [object]$Value,
    [string]$Label
  )
  if ($null -eq $Value -or [string]::IsNullOrWhiteSpace([string]$Value)) {
    Add-Failure "$Label is empty"
  } else {
    Write-Pass "$Label present"
  }
}

function Assert-ApiDataProperty {
  param(
    [object]$Payload,
    [string]$Label
  )
  if ($null -eq $Payload -or $null -eq $Payload.PSObject.Properties["data"]) {
    Add-Failure "$Label is missing the data property"
  } else {
    Write-Pass "$Label returned a data property"
  }
}

function Assert-PathExists {
  param(
    [object]$Value,
    [string]$Label
  )
  if ($null -eq $Value -or [string]::IsNullOrWhiteSpace([string]$Value)) {
    Add-Failure "$Label is empty"
    return
  }
  $path = [string]$Value
  if (-not [System.IO.Path]::IsPathRooted($path)) {
    Write-Warn "$Label is relative: $path; verify it against the app working directory"
  } elseif (Test-Path -LiteralPath $path) {
    Write-Pass "$Label exists: $path"
  } else {
    Add-Failure "$Label does not exist: $path"
  }
}

try {
  if ($StartReleaseExe) {
    if ($ExpectedPort -le 0) {
      throw "-StartReleaseExe requires -ExpectedPort to be a positive port."
    }
    $BaseUrl = "http://127.0.0.1:$ExpectedPort"
    $runtimeFull = Assert-RepoPath $RuntimeDir
    Remove-RepoItem $runtimeFull
    New-Item -ItemType Directory -Force $runtimeFull | Out-Null
    $env:RELAYCHECK_NO_OPEN = "1"
    $env:RELAYCHECK_PORT = [string]$ExpectedPort
    $startedProcess = Start-Process -FilePath $ReleaseExe -WorkingDirectory $runtimeFull -WindowStyle Hidden -PassThru
    Wait-ApiReady (Join-ApiUrl "/api/health")
  }

  Write-Host "RelayCheck operator acceptance"
  Write-Host "BaseUrl: $BaseUrl"
  Write-Host "ExpectedPort: $ExpectedPort"

  Write-Step "Health endpoint"
  $health = Invoke-ApiData "/api/health"
  $healthStatus = [string](Get-PropertyValue $health "status")
  if ($healthStatus -eq "ok") {
    Write-Pass "/api/health status is ok"
  } elseif ($healthStatus -eq "degraded" -and $AllowDegradedHealth) {
    Write-Warn "/api/health status is degraded and explicitly allowed"
  } else {
    Add-Failure "/api/health status is $healthStatus"
  }

  $healthChecks = @(Get-PropertyValue $health "checks")
  foreach ($check in $healthChecks) {
    $checkID = [string](Get-PropertyValue $check "id")
    $checkStatus = [string](Get-PropertyValue $check "status")
    $checkMessage = [string](Get-PropertyValue $check "message")
    if ($checkStatus -eq "error") {
      Add-Failure "health check $checkID is error: $checkMessage"
    } elseif ($checkStatus -eq "warning") {
      Write-Warn "health check $checkID is warning: $checkMessage"
    }
  }

  Write-Step "System status"
  $status = Invoke-ApiData "/api/system/status"
  $productName = [string](Get-PropertyValue $status "productName")
  if ($productName -eq "RelayCheck Desktop") {
    Write-Pass "product name is RelayCheck Desktop"
  } else {
    Add-Failure "unexpected product name: $productName"
  }

  $actualPort = Get-PropertyValue $status "port"
  if ($ExpectedPort -gt 0 -and [int]$actualPort -ne $ExpectedPort) {
    Add-Failure "running port is $actualPort, expected $ExpectedPort"
  } else {
    Write-Pass "running port is $actualPort"
  }

  $portConflict = [bool](Get-PropertyValue $status "portConflict")
  if ($portConflict -and -not $AllowPortConflict) {
    $preferredPort = Get-PropertyValue $status "preferredPort"
    Add-Failure "port conflict reported: preferred=$preferredPort actual=$actualPort"
  } elseif ($portConflict) {
    Write-Warn "port conflict reported and explicitly allowed"
  } else {
    Write-Pass "no port conflict reported"
  }

  Assert-PathExists (Get-PropertyValue $status "databasePath") "database path"
  Assert-PathExists (Get-PropertyValue $status "backupDir") "backup directory"
  Assert-NotBlank (Get-PropertyValue $status "scheduler") "scheduler status"
  Assert-NotBlank (Get-PropertyValue $status "lastDiagnostics") "diagnostics summary"

  Write-Step "Read-only API shape checks"
  Assert-ApiDataProperty (Invoke-ApiPayload "/api/channels") "/api/channels"
  Assert-ApiDataProperty (Invoke-ApiPayload "/api/scheduler/next-runs") "/api/scheduler/next-runs"
  Assert-ApiDataProperty (Invoke-ApiPayload "/api/scheduler/calendar?days=2") "/api/scheduler/calendar"

  if ($failures.Count -gt 0) {
    Write-Host ""
    foreach ($failure in $failures) {
      Write-Host " - $failure"
    }
    throw "$($failures.Count) operator acceptance check(s) failed."
  }

  Write-Host ""
  Write-Host "Operator acceptance passed."
} finally {
  if ($startedProcess -and -not $startedProcess.HasExited) {
    Stop-Process -Id $startedProcess.Id -Force -ErrorAction SilentlyContinue
  }
  if ($StartReleaseExe) {
    $env:RELAYCHECK_NO_OPEN = $oldNoOpen
    $env:RELAYCHECK_PORT = $oldPort
    Remove-RepoItem $RuntimeDir
    Remove-RepoDirectoryIfEmpty (Join-Path $RepoRoot ".tmp")
  }
}
