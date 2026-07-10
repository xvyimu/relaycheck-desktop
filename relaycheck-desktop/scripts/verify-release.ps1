[CmdletBinding()]
param(
  [string]$ProxyUrl = $env:RELAYCHECK_PROXY,
  [switch]$SkipGoVulnCheck,
  [switch]$SkipBrowserSmoke
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$FrontendDir = Join-Path $RepoRoot "frontend"
$RuntimeRoot = Join-Path $RepoRoot ".tmp\verify-release"
$RuntimeDir = Join-Path $RuntimeRoot "runtime"
$ReleaseExe = Join-Path $RepoRoot "dist\relaycheck.exe"

$startedProcesses = New-Object System.Collections.Generic.List[System.Diagnostics.Process]
$oldNoOpen = $env:RELAYCHECK_NO_OPEN
$oldPort = $env:RELAYCHECK_PORT
$oldHttpProxy = $env:HTTP_PROXY
$oldHttpsProxy = $env:HTTPS_PROXY
$ownsPort3001 = $false
$ownsPort5173 = $false

function Write-Step {
  param([string]$Name)
  Write-Host ""
  Write-Host "==> $Name"
}

function Invoke-Checked {
  param(
    [string]$Name,
    [string]$WorkingDirectory,
    [string]$FilePath,
    [string[]]$Arguments
  )

  Write-Step $Name
  Push-Location $WorkingDirectory
  try {
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
      throw "$Name failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

function Assert-WorkspacePath {
  param([string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  if (-not $full.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to operate outside repo: $full"
  }
  return $full
}

function Remove-WorkspaceItem {
  param([string]$Path)

  $full = Assert-WorkspacePath $Path
  if (Test-Path -LiteralPath $full) {
    Remove-Item -LiteralPath $full -Recurse -Force
  }
}

function Remove-WorkspaceDirectoryIfEmpty {
  param([string]$Path)

  $full = Assert-WorkspacePath $Path
  if ((Test-Path -LiteralPath $full) -and -not (Get-ChildItem -LiteralPath $full -Force -ErrorAction SilentlyContinue)) {
    Remove-Item -LiteralPath $full -Force
  }
}

function Assert-PortFree {
  param([int]$Port)

  $conn = Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  if ($conn) {
    $pids = ($conn | Select-Object -ExpandProperty OwningProcess -Unique) -join ","
    throw "Port $Port is already listening on 127.0.0.1 (pid: $pids)"
  }
}

function Wait-HttpOk {
  param(
    [string]$Url,
    [int]$TimeoutSeconds = 30
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    try {
      $response = Invoke-WebRequest -UseBasicParsing $Url -TimeoutSec 2
      if ($response.StatusCode -eq 200) {
        return $response
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  } while ((Get-Date) -lt $deadline)

  throw "Timed out waiting for $Url"
}

function Start-TrackedProcess {
  param(
    [string]$Name,
    [string]$FilePath,
    [string[]]$Arguments,
    [string]$WorkingDirectory
  )

  Write-Step "Start $Name"
  if ($Arguments.Count -gt 0) {
    $process = Start-Process -FilePath $FilePath -ArgumentList $Arguments -WorkingDirectory $WorkingDirectory -WindowStyle Hidden -PassThru
  } else {
    $process = Start-Process -FilePath $FilePath -WorkingDirectory $WorkingDirectory -WindowStyle Hidden -PassThru
  }
  $startedProcesses.Add($process)
  Write-Host "$Name pid=$($process.Id)"
  return $process
}

function Stop-TrackedProcesses {
  for ($i = $startedProcesses.Count - 1; $i -ge 0; $i--) {
    $process = $startedProcesses[$i]
    try {
      if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
      }
    } catch {
      Write-Warning "Failed to stop pid=$($process.Id): $($_.Exception.Message)"
    }
  }
}

function Stop-OwnedPortListeners {
  param([int]$Port)

  $conn = Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  if (-not $conn) {
    return
  }
  foreach ($listenerPid in ($conn | Select-Object -ExpandProperty OwningProcess -Unique)) {
    try {
      Stop-Process -Id $listenerPid -Force -ErrorAction SilentlyContinue
    } catch {
      Write-Warning "Failed to stop listener on port $Port pid=${listenerPid}: $($_.Exception.Message)"
    }
  }
}

try {
  Write-Host "RelayCheck release verification"
  Write-Host "Repo: $RepoRoot"

  Invoke-Checked "Frontend unit tests" $FrontendDir "npm" @("test")
  Invoke-Checked "Go tests" $RepoRoot "go" @("test", "-mod=vendor", "-count=1", "./...")
  Invoke-Checked "Go vet" $RepoRoot "go" @("vet", "-mod=vendor", "./...")
  Invoke-Checked "Frontend build" $FrontendDir "npm" @("run", "build")
  New-Item -ItemType Directory -Force (Split-Path -Parent $ReleaseExe) | Out-Null
  Invoke-Checked "Windows release binary build" $RepoRoot "go" @("build", "-mod=vendor", "-ldflags=-H windowsgui", "-o", "dist\relaycheck.exe", ".")
  Invoke-Checked "npm audit" $FrontendDir "npm" @("audit", "--audit-level=low")

  if (-not $SkipGoVulnCheck) {
    Write-Step "Go vulnerability scan"
    if ($ProxyUrl) {
      $env:HTTP_PROXY = $ProxyUrl
      $env:HTTPS_PROXY = $ProxyUrl
      Write-Host "Using proxy for govulncheck: $ProxyUrl"
    }
    Push-Location $RepoRoot
    try {
      if (Get-Command govulncheck -ErrorAction SilentlyContinue) {
        & govulncheck ./...
      } else {
        & go run golang.org/x/vuln/cmd/govulncheck@latest ./...
      }
      if ($LASTEXITCODE -ne 0) {
        throw "Go vulnerability scan failed with exit code $LASTEXITCODE"
      }
    } finally {
      Pop-Location
      $env:HTTP_PROXY = $oldHttpProxy
      $env:HTTPS_PROXY = $oldHttpsProxy
    }
  } else {
    Write-Host "Skipping Go vulnerability scan by request."
  }

  Write-Step "Release binary health smoke"
  Assert-PortFree 3001
  $ownsPort3001 = $true
  Remove-WorkspaceItem $RuntimeRoot
  New-Item -ItemType Directory -Force $RuntimeDir | Out-Null
  $env:RELAYCHECK_NO_OPEN = "1"
  $env:RELAYCHECK_PORT = "3001"
  $null = Start-TrackedProcess "relaycheck.exe" $ReleaseExe @() $RuntimeDir
  $health = Wait-HttpOk "http://127.0.0.1:3001/api/health" 30
  Write-Host "Health OK: $($health.StatusCode)"

  $channels = Wait-HttpOk "http://127.0.0.1:3001/api/channels" 10
  $payload = $channels.Content | ConvertFrom-Json
  if (-not $payload.ok) {
    throw "/api/channels did not return ok=true"
  }
  if ($null -eq $payload.data) {
    throw "/api/channels returned data=null; expected an array"
  }
  Write-Host "Fresh DB /api/channels shape OK"

  if (-not $SkipBrowserSmoke) {
    Write-Step "Browser smoke"
    Assert-PortFree 5173
    $ownsPort5173 = $true
    $vite = Start-TrackedProcess "vite dev server" "cmd.exe" @("/c", "npm", "run", "dev", "--", "--host", "127.0.0.1", "--port", "5173") $FrontendDir
    $null = $vite
    $null = Wait-HttpOk "http://127.0.0.1:5173" 30
    Invoke-Checked "Scheduler layout smoke" $FrontendDir "npm" @("run", "smoke:schedules")
    Invoke-Checked "Navigation intent smoke" $FrontendDir "npm" @("run", "smoke")
  } else {
    Write-Host "Skipping browser smoke by request."
  }

  Invoke-Checked "Whitespace check" $RepoRoot "git" @("diff", "--check")

  Write-Host ""
  Write-Host "Release verification passed."
} finally {
  Stop-TrackedProcesses
  if ($ownsPort5173) {
    Stop-OwnedPortListeners 5173
  }
  if ($ownsPort3001) {
    Stop-OwnedPortListeners 3001
  }
  $env:RELAYCHECK_NO_OPEN = $oldNoOpen
  $env:RELAYCHECK_PORT = $oldPort
  $env:HTTP_PROXY = $oldHttpProxy
  $env:HTTPS_PROXY = $oldHttpsProxy
  Remove-WorkspaceItem $RuntimeRoot
  Remove-WorkspaceDirectoryIfEmpty (Join-Path $RepoRoot ".tmp")
  Remove-WorkspaceItem (Join-Path $FrontendDir "verify-canary.txt")
  Remove-WorkspaceItem (Join-Path $FrontendDir "verify-nav-output.txt")
}
