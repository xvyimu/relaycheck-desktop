[CmdletBinding()]
param(
  [string]$OutputDir = "",
  [switch]$SkipBuild,
  [switch]$AllowDirty
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$FrontendDir = Join-Path $RepoRoot "frontend"
$ReleaseExe = Join-Path $RepoRoot "dist\relaycheck.exe"
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path $RepoRoot "dist\releases"
}

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

function Get-ProductVersion {
  $routesPath = Join-Path $RepoRoot "internal\core\routes.go"
  $routes = Get-Content -LiteralPath $routesPath -Raw
  $match = [regex]::Match($routes, 'productVersion\s*=\s*"([^"]+)"')
  if (-not $match.Success) {
    throw "Could not find productVersion in internal\core\routes.go"
  }
  return $match.Groups[1].Value
}

function Get-ReleaseLdflags {
  param(
    [string]$Version,
    [string]$Commit
  )
  $buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  $parts = @(
    "-H=windowsgui",
    "-s",
    "-w",
    "-X=relaycheck-desktop/internal/core.productVersion=$Version",
    "-X=relaycheck-desktop/internal/core.buildTime=$buildTime",
    "-X=relaycheck-desktop/internal/core.gitCommit=$Commit"
  )
  return ($parts -join " ")
}

function Get-GitValue {
  param([string[]]$Arguments)

  Push-Location $RepoRoot
  try {
    $value = & git @Arguments
    if ($LASTEXITCODE -ne 0) {
      throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
    return [string]$value
  } finally {
    Pop-Location
  }
}

function Copy-PackageFile {
  param(
    [string]$Source,
    [string]$Destination
  )

  $sourceFull = Assert-RepoPath $Source
  $destinationFull = Assert-RepoPath $Destination
  $destinationDir = Split-Path -Parent $destinationFull
  New-Item -ItemType Directory -Force $destinationDir | Out-Null
  Copy-Item -LiteralPath $sourceFull -Destination $destinationFull -Force
}

function New-ChecksumEntry {
  param(
    [string]$Path,
    [string]$Root
  )

  $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $Path
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd("\", "/")
  $pathFull = [System.IO.Path]::GetFullPath($Path)
  if (-not $pathFull.StartsWith($rootFull, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Checksum path is outside package root: $pathFull"
  }
  $relative = $pathFull.Substring($rootFull.Length).TrimStart("\", "/").Replace("\", "/")
  [pscustomobject]@{
    path   = $relative
    sha256 = $hash.Hash.ToLowerInvariant()
    bytes  = (Get-Item -LiteralPath $Path).Length
  }
}

Write-Host "RelayCheck release packaging"
Write-Host "Repo: $RepoRoot"

$gitStatus = Get-GitValue @("status", "--porcelain")
$isDirty = -not [string]::IsNullOrWhiteSpace($gitStatus)
if ($isDirty -and -not $AllowDirty) {
  throw "Working tree is dirty. Commit or stash changes before packaging, or pass -AllowDirty for development verification."
}

$productVersion = Get-ProductVersion
$commit = Get-GitValue @("rev-parse", "--short=12", "HEAD")
$commitFull = Get-GitValue @("rev-parse", "HEAD")

if (-not $SkipBuild) {
  Invoke-Checked "Frontend build" $FrontendDir "npm" @("run", "build")
  New-Item -ItemType Directory -Force (Split-Path -Parent $ReleaseExe) | Out-Null
  $ldflags = Get-ReleaseLdflags -Version $productVersion -Commit $commit
  Invoke-Checked "Windows release binary build" $RepoRoot "go" @("build", "-mod=vendor", "-ldflags=$ldflags", "-o", "dist\relaycheck.exe", ".")
}

if (-not (Test-Path -LiteralPath $ReleaseExe)) {
  throw "Release executable not found: $ReleaseExe"
}

$timestamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss")
$packageName = "relaycheck-desktop-$($productVersion.TrimStart('v'))-$commit-$timestamp"
$outputFull = Assert-RepoPath $OutputDir
$packageDir = Join-Path $outputFull $packageName
$zipPath = Join-Path $outputFull "$packageName.zip"

Write-Step "Prepare package directory"
Remove-RepoItem $packageDir
if (Test-Path -LiteralPath $zipPath) {
  Remove-RepoItem $zipPath
}
New-Item -ItemType Directory -Force $packageDir | Out-Null

Copy-PackageFile $ReleaseExe (Join-Path $packageDir "relaycheck.exe")
Copy-PackageFile (Join-Path $RepoRoot "docs\LAUNCH_READINESS.md") (Join-Path $packageDir "docs\LAUNCH_READINESS.md")
Copy-PackageFile (Join-Path $RepoRoot "docs\OPERATOR_RUNBOOK.md") (Join-Path $packageDir "docs\OPERATOR_RUNBOOK.md")
Copy-PackageFile (Join-Path $RepoRoot "docs\OPERATOR_ACCEPTANCE_RECORD.md") (Join-Path $packageDir "docs\OPERATOR_ACCEPTANCE_RECORD.md")
Copy-PackageFile (Join-Path $RepoRoot "scripts\operator-acceptance.ps1") (Join-Path $packageDir "scripts\operator-acceptance.ps1")
Copy-PackageFile (Join-Path $RepoRoot "scripts\operator-launch.ps1") (Join-Path $packageDir "scripts\operator-launch.ps1")
Copy-PackageFile (Join-Path $RepoRoot "scripts\operator-monitor.ps1") (Join-Path $packageDir "scripts\operator-monitor.ps1")
Copy-PackageFile (Join-Path $RepoRoot "scripts\verify-package.ps1") (Join-Path $packageDir "scripts\verify-package.ps1")
Copy-PackageFile (Join-Path $RepoRoot "README.md") (Join-Path $packageDir "README.md")

$exeInPackage = Join-Path $packageDir "relaycheck.exe"
$includedFiles = @(
  $exeInPackage,
  (Join-Path $packageDir "README.md"),
  (Join-Path $packageDir "docs\LAUNCH_READINESS.md"),
  (Join-Path $packageDir "docs\OPERATOR_RUNBOOK.md"),
  (Join-Path $packageDir "docs\OPERATOR_ACCEPTANCE_RECORD.md"),
  (Join-Path $packageDir "scripts\operator-acceptance.ps1"),
  (Join-Path $packageDir "scripts\operator-launch.ps1"),
  (Join-Path $packageDir "scripts\operator-monitor.ps1"),
  (Join-Path $packageDir "scripts\verify-package.ps1")
)

$checksums = foreach ($file in $includedFiles) {
  New-ChecksumEntry $file $packageDir
}

$manifest = [ordered]@{
  product          = "RelayCheck Desktop"
  version          = $productVersion
  gitCommit        = $commitFull
  gitCommitShort   = $commit
  gitDirty         = $isDirty
  generatedAtUtc   = (Get-Date).ToUniversalTime().ToString("o")
  platform         = "windows"
  entrypoint       = "relaycheck.exe"
  releaseGate      = "Run scripts\\verify-release.ps1 before packaging."
  operatorRunbook  = "docs/OPERATOR_RUNBOOK.md"
  operatorAcceptanceRecord = "docs/OPERATOR_ACCEPTANCE_RECORD.md"
  operatorLaunch   = "scripts/operator-launch.ps1"
  operatorMonitor  = "scripts/operator-monitor.ps1"
  packageVerifier  = "scripts/verify-package.ps1"
  launchReadiness  = "docs/LAUNCH_READINESS.md"
  files            = $checksums
}

$manifestPath = Join-Path $packageDir "manifest.json"
$manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding UTF8

$allChecksums = foreach ($file in @($includedFiles + $manifestPath)) {
  New-ChecksumEntry $file $packageDir
}
$checksumLines = foreach ($entry in $allChecksums) {
  "$($entry.sha256)  $($entry.path)"
}
$checksumPath = Join-Path $packageDir "checksums.sha256"
Set-Content -LiteralPath $checksumPath -Value $checksumLines -Encoding ASCII

Write-Step "Create zip"
Push-Location $packageDir
try {
  Compress-Archive -Path "*" -DestinationPath $zipPath -Force
} finally {
  Pop-Location
}
$zipHash = Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath
$zipSha256 = $zipHash.Hash.ToLowerInvariant()
$zipChecksumPath = "$zipPath.sha256"
Set-Content -LiteralPath $zipChecksumPath -Value "$zipSha256  $(Split-Path -Leaf $zipPath)" -Encoding ASCII

$packageInfo = [ordered]@{
  packageDir     = $packageDir
  zipPath        = $zipPath
  zipChecksumPath = $zipChecksumPath
  zipSha256      = $zipSha256
  version        = $productVersion
  commit         = $commitFull
  dirty          = $isDirty
}

Write-Host ""
Write-Host "Release package created:"
Write-Host "Package dir: $packageDir"
Write-Host "Zip: $zipPath"
Write-Host "Zip SHA256: $zipSha256"
Write-Host "Zip checksum file: $zipChecksumPath"

$packageInfo | ConvertTo-Json -Depth 4
