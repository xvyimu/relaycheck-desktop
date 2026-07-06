[CmdletBinding()]
param(
  [string]$PackageDir = "",
  [string]$BaseUrl = "http://127.0.0.1:3001",
  [int]$ExpectedPort = 3001,
  [int]$TimeoutSeconds = 15,
  [int[]]$IntervalsSeconds = @(),
  [int]$SampleCount = 0,
  [int]$IntervalSeconds = 0,
  [switch]$AllowDegradedHealth,
  [switch]$AllowPortConflict,
  [switch]$AllowCriticalActionCenter,
  [string]$RecordPath = "",
  [string]$JsonRecordPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$failures = New-Object System.Collections.Generic.List[string]
$warnings = New-Object System.Collections.Generic.List[string]
$samples = New-Object System.Collections.Generic.List[object]

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
  $warnings.Add($Message) | Out-Null
  Write-Host "[WARN] $Message"
}

function Add-Failure {
  param([string]$Message)
  $failures.Add($Message) | Out-Null
  Write-Host "[FAIL] $Message"
}

function Add-SampleFailure {
  param(
    [System.Collections.Generic.List[string]]$List,
    [string]$Message
  )
  $List.Add($Message) | Out-Null
  Add-Failure $Message
}

function Add-SampleWarning {
  param(
    [System.Collections.Generic.List[string]]$List,
    [string]$Message
  )
  $List.Add($Message) | Out-Null
  Write-Warn $Message
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
    [string]$Url,
    [string]$Path
  )
  return $Url.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-ApiPayload {
  param([string]$Path)

  $url = Join-ApiUrl $BaseUrl $Path
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

  if ((Get-PropertyValue $payload "ok") -ne $true) {
    $errorMessage = [string](Get-PropertyValue $payload "error")
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
  if ($null -eq $data) {
    throw "$Path returned no data property"
  }
  Write-Output -NoEnumerate $data
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

function Format-Cell {
  param([object]$Value)
  if ($null -eq $Value) {
    return ""
  }
  return ([string]$Value).Replace("|", "\|").Replace([string][char]13, " ").Replace([string][char]10, " ")
}

function Get-ArrayCount {
  param([object]$Value)
  if ($null -eq $Value) {
    return 0
  }
  return @($Value).Count
}

function Get-MonitorIntervals {
  if ($IntervalsSeconds.Count -gt 0) {
    $items = @($IntervalsSeconds)
  } elseif ($SampleCount -gt 0) {
    if ($IntervalSeconds -lt 0) {
      throw "-IntervalSeconds must be non-negative."
    }
    $items = @()
    for ($i = 0; $i -lt $SampleCount; $i++) {
      $items += ($i * $IntervalSeconds)
    }
  } else {
    $items = @(0, 300, 900, 1800, 3600)
  }

  foreach ($item in $items) {
    if ($item -lt 0) {
      throw "monitor interval must be non-negative: $item"
    }
  }
  return @($items | Sort-Object -Unique)
}

function Read-Manifest {
  param([string]$PackageRoot)

  $manifestPath = Join-Path $PackageRoot "manifest.json"
  if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    return $null
  }
  return Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
}

function Test-Health {
  param(
    [System.Collections.Generic.List[string]]$SampleFailures,
    [System.Collections.Generic.List[string]]$SampleWarnings
  )

  $health = Invoke-ApiData "/api/health"
  $healthStatus = [string](Get-PropertyValue $health "status")
  if ($healthStatus -eq "ok") {
    Write-Pass "/api/health status is ok"
  } elseif ($healthStatus -eq "degraded" -and $AllowDegradedHealth) {
    Add-SampleWarning $SampleWarnings "/api/health status is degraded and explicitly allowed"
  } else {
    Add-SampleFailure $SampleFailures "/api/health status is $healthStatus"
  }

  foreach ($check in @(Get-PropertyValue $health "checks")) {
    $checkID = [string](Get-PropertyValue $check "id")
    $checkStatus = [string](Get-PropertyValue $check "status")
    $checkMessage = [string](Get-PropertyValue $check "message")
    if ($checkStatus -in @("fail", "failed", "error")) {
      Add-SampleFailure $SampleFailures ("health check " + $checkID + " is " + $checkStatus + ": " + $checkMessage)
    } elseif ($checkStatus -in @("warn", "warning")) {
      Add-SampleWarning $SampleWarnings ("health check " + $checkID + " is " + $checkStatus + ": " + $checkMessage)
    }
  }
  return $healthStatus
}

function Test-SystemStatus {
  param(
    [System.Collections.Generic.List[string]]$SampleFailures,
    [System.Collections.Generic.List[string]]$SampleWarnings
  )

  $status = Invoke-ApiData "/api/system/status"
  $productName = [string](Get-PropertyValue $status "productName")
  if ($productName -ne "RelayCheck Desktop") {
    Add-SampleFailure $SampleFailures "unexpected product name: $productName"
  }

  $actualPortValue = Get-PropertyValue $status "port"
  $actualPort = 0
  try {
    $actualPort = [int]$actualPortValue
  } catch {
    Add-SampleFailure $SampleFailures "system status port is invalid: $actualPortValue"
  }

  if ($ExpectedPort -gt 0 -and $actualPort -ne $ExpectedPort) {
    Add-SampleFailure $SampleFailures "running port is $actualPort, expected $ExpectedPort"
  }

  $portConflict = [bool](Get-PropertyValue $status "portConflict")
  if ($portConflict -and -not $AllowPortConflict) {
    $preferredPort = Get-PropertyValue $status "preferredPort"
    Add-SampleFailure $SampleFailures "port conflict reported: preferred=$preferredPort actual=$actualPort"
  } elseif ($portConflict) {
    Add-SampleWarning $SampleWarnings "port conflict reported and explicitly allowed"
  }

  foreach ($field in @("databasePath", "backupDir")) {
    if ([string]::IsNullOrWhiteSpace([string](Get-PropertyValue $status $field))) {
      Add-SampleFailure $SampleFailures "system status $field is empty"
    }
  }

  $diagnostics = Get-PropertyValue $status "lastDiagnostics"
  $diagnosticsOverall = [string](Get-PropertyValue $diagnostics "overall")
  if ($diagnosticsOverall -in @("critical", "danger")) {
    Add-SampleFailure $SampleFailures "diagnostics overall is $diagnosticsOverall"
  } elseif ($diagnosticsOverall -eq "warning") {
    Add-SampleWarning $SampleWarnings "diagnostics overall is warning"
  }

  $scheduler = Get-PropertyValue $status "scheduler"
  $schedulerJobCount = Get-ArrayCount (Get-PropertyValue $scheduler "jobs")
  [pscustomobject]@{
    port               = $actualPort
    portConflict       = $portConflict
    diagnosticsOverall = $diagnosticsOverall
    schedulerJobCount  = $schedulerJobCount
  }
}

function Test-ProjectionEndpoint {
  param(
    [string]$Path,
    [string]$Label,
    [System.Collections.Generic.List[string]]$SampleFailures
  )

  $data = Invoke-ApiData $Path
  $generatedAt = [string](Get-PropertyValue $data "generatedAt")
  if ([string]::IsNullOrWhiteSpace($generatedAt)) {
    Add-SampleFailure $SampleFailures "$Label generatedAt is empty"
  }
  return Get-ArrayCount (Get-PropertyValue $data "items")
}

function Test-ActionCenter {
  param(
    [System.Collections.Generic.List[string]]$SampleFailures,
    [System.Collections.Generic.List[string]]$SampleWarnings
  )

  $center = Invoke-ApiData "/api/system/action-center"
  $overall = [string](Get-PropertyValue $center "overall")
  $criticalCount = 0
  foreach ($item in @(Get-PropertyValue $center "items")) {
    $level = [string](Get-PropertyValue $item "level")
    if ($level -in @("danger", "critical")) {
      $criticalCount++
    }
  }

  if (($overall -in @("danger", "critical") -or $criticalCount -gt 0) -and -not $AllowCriticalActionCenter) {
    Add-SampleFailure $SampleFailures "action center has $criticalCount critical item(s); overall=$overall"
  } elseif ($overall -eq "warning") {
    Add-SampleWarning $SampleWarnings "action center overall is warning"
  }

  [pscustomobject]@{
    overall       = $overall
    criticalCount = $criticalCount
  }
}

function Invoke-MonitorSample {
  param(
    [int]$Index,
    [int]$ScheduledElapsedSeconds,
    [datetime]$StartedAt
  )

  $sampleFailures = New-Object System.Collections.Generic.List[string]
  $sampleWarnings = New-Object System.Collections.Generic.List[string]
  $observedAt = Get-Date
  $elapsed = [int][Math]::Round(($observedAt - $StartedAt).TotalSeconds)

  Write-Step "Sample $Index at +$ScheduledElapsedSeconds second(s)"

  $healthStatus = "unknown"
  $systemSummary = [pscustomobject]@{
    port               = 0
    portConflict       = $false
    diagnosticsOverall = ""
    schedulerJobCount  = 0
  }
  $nextRunCount = 0
  $calendarItemCount = 0
  $actionSummary = [pscustomobject]@{
    overall       = ""
    criticalCount = 0
  }

  try {
    $healthStatus = Test-Health $sampleFailures $sampleWarnings
  } catch {
    Add-SampleFailure $sampleFailures $_.Exception.Message
  }

  try {
    $systemSummary = Test-SystemStatus $sampleFailures $sampleWarnings
  } catch {
    Add-SampleFailure $sampleFailures $_.Exception.Message
  }

  try {
    $nextRunCount = Test-ProjectionEndpoint "/api/scheduler/next-runs" "/api/scheduler/next-runs" $sampleFailures
  } catch {
    Add-SampleFailure $sampleFailures $_.Exception.Message
  }

  try {
    $calendarItemCount = Test-ProjectionEndpoint "/api/scheduler/calendar?days=2" "/api/scheduler/calendar" $sampleFailures
  } catch {
    Add-SampleFailure $sampleFailures $_.Exception.Message
  }

  try {
    $actionSummary = Test-ActionCenter $sampleFailures $sampleWarnings
  } catch {
    Add-SampleFailure $sampleFailures $_.Exception.Message
  }

  $result = "pass"
  if ($sampleFailures.Count -gt 0) {
    $result = "fail"
  }

  [pscustomobject]@{
    index                   = $Index
    scheduledElapsedSeconds = $ScheduledElapsedSeconds
    observedAtUtc           = $observedAt.ToUniversalTime().ToString("o")
    elapsedSeconds          = $elapsed
    result                  = $result
    healthStatus            = $healthStatus
    systemPort              = $systemSummary.port
    portConflict            = $systemSummary.portConflict
    diagnosticsOverall      = $systemSummary.diagnosticsOverall
    schedulerJobCount       = $systemSummary.schedulerJobCount
    nextRunCount            = $nextRunCount
    calendarItemCount       = $calendarItemCount
    actionCenterOverall     = $actionSummary.overall
    criticalActionCount     = $actionSummary.criticalCount
    warnings                = $sampleWarnings.ToArray()
    failures                = $sampleFailures.ToArray()
  }
}

function Write-MonitorRecords {
  param(
    [string]$MarkdownPath,
    [string]$JsonPath,
    [object]$Manifest,
    [string]$PackageRoot,
    [string]$Result,
    [int[]]$Intervals,
    [object[]]$SampleItems
  )

  $version = ""
  $gitCommit = ""
  $gitDirty = ""
  if ($null -ne $Manifest) {
    $version = [string](Get-PropertyValue $Manifest "version")
    $gitCommit = [string](Get-PropertyValue $Manifest "gitCommit")
    $gitDirty = [string][bool](Get-PropertyValue $Manifest "gitDirty")
  }

  $jsonRecord = [ordered]@{
    generatedAtUtc   = (Get-Date).ToUniversalTime().ToString("o")
    packageRoot      = $PackageRoot
    baseUrl          = $BaseUrl
    expectedPort     = $ExpectedPort
    version          = $version
    gitCommit        = $gitCommit
    gitDirty         = $gitDirty
    intervalsSeconds = $Intervals
    result           = $Result
    failureCount     = $failures.Count
    warningCount     = $warnings.Count
    samples          = $SampleItems
  }

  $recordDir = Split-Path -Parent $MarkdownPath
  New-Item -ItemType Directory -Force $recordDir | Out-Null
  $jsonDir = Split-Path -Parent $JsonPath
  New-Item -ItemType Directory -Force $jsonDir | Out-Null
  $jsonRecord | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $JsonPath -Encoding UTF8

  $lines = @(
    "# Operator Monitor Record",
    "",
    "Generated at UTC: $($jsonRecord.generatedAtUtc)",
    "Package root: $PackageRoot",
    "Base URL: $BaseUrl",
    "Expected port: $ExpectedPort",
    "Result: $Result",
    "JSON record: $JsonPath",
    "",
    "## Package",
    "",
    "| Field | Value |",
    "| --- | --- |",
    "| Version | $(Format-Cell $version) |",
    "| Git commit | $(Format-Cell $gitCommit) |",
    "| Git dirty | $(Format-Cell $gitDirty) |",
    "",
    "## Samples",
    "",
    "| # | Target | Observed UTC | Result | Health | Port | Diagnostics | Next runs | Calendar | Action center | Critical actions |",
    "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"
  )

  foreach ($sample in $SampleItems) {
    $lines += "| $($sample.index) | +$($sample.scheduledElapsedSeconds)s | $(Format-Cell $sample.observedAtUtc) | $(Format-Cell $sample.result) | $(Format-Cell $sample.healthStatus) | $(Format-Cell $sample.systemPort) | $(Format-Cell $sample.diagnosticsOverall) | $(Format-Cell $sample.nextRunCount) | $(Format-Cell $sample.calendarItemCount) | $(Format-Cell $sample.actionCenterOverall) | $(Format-Cell $sample.criticalActionCount) |"
  }

  if ($warnings.Count -gt 0) {
    $lines += @("", "## Warnings", "")
    foreach ($warning in $warnings) {
      $lines += "- $(Format-Cell $warning)"
    }
  }

  if ($failures.Count -gt 0) {
    $lines += @("", "## Failures", "")
    foreach ($failure in $failures) {
      $lines += "- $(Format-Cell $failure)"
    }
  }

  $lines += @(
    "",
    "## Notes",
    "",
    "- This record is generated by scripts\\operator-monitor.ps1.",
    "- It stores health/status counts and failure summaries only; do not paste passwords, cookies, bearer tokens, API keys, exported .rczip passwords, or private database contents here.",
    "- A failed monitor result is a launch hold or rollback signal unless an operator records an accepted warning separately."
  )

  Set-Content -LiteralPath $MarkdownPath -Value $lines -Encoding UTF8
}

if ($TimeoutSeconds -le 0) {
  throw "-TimeoutSeconds must be positive."
}
if ($ExpectedPort -lt 0) {
  throw "-ExpectedPort must be 0 or a positive port."
}
if ($SampleCount -lt 0) {
  throw "-SampleCount must be non-negative."
}

if ([string]::IsNullOrWhiteSpace($PackageDir)) {
  $PackageDir = Join-Path $PSScriptRoot ".."
}
$packageRoot = (Resolve-Path -LiteralPath $PackageDir).Path
$manifest = Read-Manifest $packageRoot
$intervals = Get-MonitorIntervals

if ([string]::IsNullOrWhiteSpace($RecordPath)) {
  $stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss")
  $RecordPath = Join-Path $packageRoot "launch-records\operator-monitor-$stamp.md"
}
$recordFull = Assert-UnderPackage $packageRoot $RecordPath "record path"

if ([string]::IsNullOrWhiteSpace($JsonRecordPath)) {
  $JsonRecordPath = [System.IO.Path]::ChangeExtension($recordFull, ".json")
}
$jsonRecordFull = Assert-UnderPackage $packageRoot $JsonRecordPath "JSON record path"

Write-Host "RelayCheck operator monitor"
Write-Host "Package root: $packageRoot"
Write-Host "BaseUrl: $BaseUrl"
Write-Host "ExpectedPort: $ExpectedPort"
Write-Host "IntervalsSeconds: $($intervals -join ',')"

$startedAt = Get-Date
$sampleIndex = 0
foreach ($targetSeconds in $intervals) {
  $targetAt = $startedAt.AddSeconds($targetSeconds)
  $remaining = [int][Math]::Ceiling(($targetAt - (Get-Date)).TotalSeconds)
  if ($remaining -gt 0) {
    Start-Sleep -Seconds $remaining
  }
  $sampleIndex++
  $sample = Invoke-MonitorSample $sampleIndex $targetSeconds $startedAt
  $samples.Add($sample) | Out-Null
}

$result = "pass"
if ($failures.Count -gt 0) {
  $result = "fail"
}

Write-MonitorRecords $recordFull $jsonRecordFull $manifest $packageRoot $result $intervals ($samples.ToArray())

if ($failures.Count -gt 0) {
  Write-Host ""
  foreach ($failure in $failures) {
    Write-Host " - $failure"
  }
  Write-Host "Monitor record: $recordFull"
  Write-Host "Monitor JSON: $jsonRecordFull"
  throw "$($failures.Count) operator monitor check(s) failed."
}

Write-Host ""
Write-Host "Operator monitor passed."
Write-Host "Monitor record: $recordFull"
Write-Host "Monitor JSON: $jsonRecordFull"
