[CmdletBinding()]
param(
  [string]$ZipPath = "",
  [string]$PackageDir = "",
  [switch]$AllowDirtyManifest,
  [switch]$KeepExtracted
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()

$Root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
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

function Assert-UnderRoot {
  param(
    [string]$Path,
    [string]$Label
  )

  $full = [System.IO.Path]::GetFullPath($Path)
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd("\", "/")
  $rootWithSeparator = $rootFull + [System.IO.Path]::DirectorySeparatorChar
  if ($full -ne $rootFull -and -not $full.StartsWith($rootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "$Label is outside verifier root: $full"
  }
  return $full
}

function Remove-RootItem {
  param([string]$Path)

  $full = Assert-UnderRoot $Path "temporary path"
  if (Test-Path -LiteralPath $full) {
    Remove-Item -LiteralPath $full -Recurse -Force
  }
}

function Normalize-PackagePath {
  param([string]$Path)
  return $Path.Replace("\", "/").TrimStart("/")
}

function Test-SafeRelativePath {
  param([string]$Path)

  if ([string]::IsNullOrWhiteSpace($Path)) {
    return $false
  }
  if ([System.IO.Path]::IsPathRooted($Path)) {
    return $false
  }
  $normalized = Normalize-PackagePath $Path
  if ($normalized -eq ".." -or $normalized.StartsWith("../") -or $normalized.Contains("/../")) {
    return $false
  }
  return $true
}

function Join-PackagePath {
  param(
    [string]$PackageRoot,
    [string]$RelativePath
  )

  if (-not (Test-SafeRelativePath $RelativePath)) {
    Add-Failure "unsafe package path: $RelativePath"
    return $null
  }

  $packageRootFull = [System.IO.Path]::GetFullPath($PackageRoot).TrimEnd("\", "/")
  $candidate = [System.IO.Path]::GetFullPath((Join-Path $packageRootFull ($RelativePath.Replace("/", "\"))))
  $rootWithSeparator = $packageRootFull + [System.IO.Path]::DirectorySeparatorChar
  if ($candidate -ne $packageRootFull -and -not $candidate.StartsWith($rootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
    Add-Failure "package path escapes package root: $RelativePath"
    return $null
  }
  return $candidate
}

function Assert-RequiredFile {
  param(
    [string]$PackageRoot,
    [string]$RelativePath
  )

  $full = Join-PackagePath $PackageRoot $RelativePath
  if ($null -eq $full) {
    return
  }
  if (Test-Path -LiteralPath $full -PathType Leaf) {
    Write-Pass "$RelativePath present"
  } else {
    Add-Failure "$RelativePath missing"
  }
}

function Read-JsonFile {
  param([string]$Path)

  try {
    return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
  } catch {
    Add-Failure "invalid JSON in $Path`: $($_.Exception.Message)"
    return $null
  }
}

function Assert-ManifestPath {
  param(
    [string]$PackageRoot,
    [object]$Manifest,
    [string]$FieldName,
    [string]$ExpectedPath
  )

  $value = [string](Get-PropertyValue $Manifest $FieldName)
  if ([string]::IsNullOrWhiteSpace($value)) {
    Add-Failure "manifest.$FieldName is empty"
    return
  }
  $normalizedValue = Normalize-PackagePath $value
  $normalizedExpected = Normalize-PackagePath $ExpectedPath
  if ($normalizedValue -ne $normalizedExpected) {
    Add-Failure "manifest.$FieldName is $value, expected $ExpectedPath"
    return
  }
  Assert-RequiredFile $PackageRoot $normalizedValue
}

function Test-ZipSidecar {
  param([string]$ZipFull)

  $sidecar = "$ZipFull.sha256"
  if (-not (Test-Path -LiteralPath $sidecar -PathType Leaf)) {
    Add-Failure "zip checksum sidecar missing: $sidecar"
    return
  }

  $line = (Get-Content -LiteralPath $sidecar | Select-Object -First 1)
  if ($line -notmatch '^([a-fA-F0-9]{64})\s+(.+)$') {
    Add-Failure "zip checksum sidecar has invalid format: $sidecar"
    return
  }

  $expectedHash = $matches[1].ToLowerInvariant()
  $expectedName = $matches[2].Trim()
  $actualName = Split-Path -Leaf $ZipFull
  $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ZipFull).Hash.ToLowerInvariant()

  if ($expectedName -ne $actualName) {
    Add-Failure "zip checksum sidecar names $expectedName, expected $actualName"
  } else {
    Write-Pass "zip checksum sidecar names the package"
  }

  if ($expectedHash -ne $actualHash) {
    Add-Failure "zip SHA256 mismatch: sidecar=$expectedHash actual=$actualHash"
  } else {
    Write-Pass "zip SHA256 matches sidecar"
  }
}

function Expand-ZipForValidation {
  param([string]$ZipFull)

  $packageName = [System.IO.Path]::GetFileNameWithoutExtension($ZipFull)
  $extractRoot = Join-Path $Root ".tmp\verify-package"
  $extractDir = Join-Path $extractRoot $packageName
  Remove-RootItem $extractDir
  New-Item -ItemType Directory -Force $extractDir | Out-Null
  Expand-Archive -LiteralPath $ZipFull -DestinationPath $extractDir -Force
  return $extractDir
}

function Get-LatestZip {
  $releaseDir = Join-Path $Root "dist\releases"
  if (-not (Test-Path -LiteralPath $releaseDir -PathType Container)) {
    throw "Release directory not found: $releaseDir"
  }
  $zip = Get-ChildItem -LiteralPath $releaseDir -Filter *.zip | Sort-Object LastWriteTime -Descending | Select-Object -First 1
  if ($null -eq $zip) {
    throw "No release zip found under $releaseDir"
  }
  return $zip.FullName
}

function Test-ChecksumFile {
  param(
    [string]$PackageRoot,
    [string[]]$RequiredChecksumPaths
  )

  $checksumPath = Join-Path $PackageRoot "checksums.sha256"
  if (-not (Test-Path -LiteralPath $checksumPath -PathType Leaf)) {
    Add-Failure "checksums.sha256 missing"
    return
  }

  $checksumByPath = @{}
  $lineNumber = 0
  foreach ($line in Get-Content -LiteralPath $checksumPath) {
    $lineNumber++
    if ([string]::IsNullOrWhiteSpace($line)) {
      continue
    }
    if ($line -notmatch '^([a-fA-F0-9]{64})\s+(.+)$') {
      Add-Failure "checksums.sha256 line $lineNumber has invalid format"
      continue
    }

    $expectedHash = $matches[1].ToLowerInvariant()
    $relativePath = Normalize-PackagePath $matches[2].Trim()
    $filePath = Join-PackagePath $PackageRoot $relativePath
    if ($null -eq $filePath) {
      continue
    }
    if (-not (Test-Path -LiteralPath $filePath -PathType Leaf)) {
      Add-Failure "checksummed file missing: $relativePath"
      continue
    }

    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $filePath).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
      Add-Failure "checksum mismatch for $relativePath"
    } else {
      Write-Pass "checksum OK: $relativePath"
    }
    $checksumByPath[$relativePath] = $expectedHash
  }

  foreach ($requiredPath in $RequiredChecksumPaths) {
    $normalized = Normalize-PackagePath $requiredPath
    if (-not $checksumByPath.ContainsKey($normalized)) {
      Add-Failure "checksums.sha256 missing entry for $normalized"
    }
  }
}

function Test-Manifest {
  param(
    [string]$PackageRoot,
    [string[]]$RequiredFiles
  )

  $manifestPath = Join-Path $PackageRoot "manifest.json"
  if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    Add-Failure "manifest.json missing"
    return
  }

  $manifest = Read-JsonFile $manifestPath
  if ($null -eq $manifest) {
    return
  }

  if ([string](Get-PropertyValue $manifest "product") -eq "RelayCheck Desktop") {
    Write-Pass "manifest product is RelayCheck Desktop"
  } else {
    Add-Failure "manifest product is not RelayCheck Desktop"
  }

  $version = [string](Get-PropertyValue $manifest "version")
  if ($version -match '^v\d+\.\d+\.\d+') {
    Write-Pass "manifest version present: $version"
  } else {
    Add-Failure "manifest version is invalid: $version"
  }

  $commit = [string](Get-PropertyValue $manifest "gitCommit")
  if ($commit -match '^[a-f0-9]{40}$') {
    Write-Pass "manifest gitCommit is a full SHA"
  } else {
    Add-Failure "manifest gitCommit is invalid: $commit"
  }

  $dirty = [bool](Get-PropertyValue $manifest "gitDirty")
  if ($dirty -and -not $AllowDirtyManifest) {
    Add-Failure "manifest gitDirty is true"
  } elseif ($dirty) {
    Write-Warn "manifest gitDirty is true and explicitly allowed"
  } else {
    Write-Pass "manifest gitDirty is false"
  }

  if ([string](Get-PropertyValue $manifest "platform") -eq "windows") {
    Write-Pass "manifest platform is windows"
  } else {
    Add-Failure "manifest platform is not windows"
  }

  Assert-ManifestPath $PackageRoot $manifest "entrypoint" "relaycheck.exe"
  Assert-ManifestPath $PackageRoot $manifest "operatorRunbook" "docs/OPERATOR_RUNBOOK.md"
  Assert-ManifestPath $PackageRoot $manifest "operatorAcceptanceRecord" "docs/OPERATOR_ACCEPTANCE_RECORD.md"
  Assert-ManifestPath $PackageRoot $manifest "operatorLaunch" "scripts/operator-launch.ps1"
  Assert-ManifestPath $PackageRoot $manifest "packageVerifier" "scripts/verify-package.ps1"
  Assert-ManifestPath $PackageRoot $manifest "launchReadiness" "docs/LAUNCH_READINESS.md"

  $manifestFiles = @(Get-PropertyValue $manifest "files")
  foreach ($entry in $manifestFiles) {
    $relativePath = [string](Get-PropertyValue $entry "path")
    $expectedHash = [string](Get-PropertyValue $entry "sha256")
    if (-not (Test-SafeRelativePath $relativePath)) {
      Add-Failure "manifest file path is unsafe: $relativePath"
      continue
    }
    if ($expectedHash -notmatch '^[a-fA-F0-9]{64}$') {
      Add-Failure "manifest file hash is invalid for $relativePath"
      continue
    }
    $filePath = Join-PackagePath $PackageRoot $relativePath
    if ($null -eq $filePath -or -not (Test-Path -LiteralPath $filePath -PathType Leaf)) {
      Add-Failure "manifest file missing: $relativePath"
      continue
    }
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $filePath).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash.ToLowerInvariant()) {
      Add-Failure "manifest file hash mismatch for $relativePath"
    }
  }

  foreach ($requiredFile in $RequiredFiles) {
    if ($requiredFile -eq "manifest.json" -or $requiredFile -eq "checksums.sha256") {
      continue
    }
    $normalized = Normalize-PackagePath $requiredFile
    $match = $false
    foreach ($entry in $manifestFiles) {
      if ((Normalize-PackagePath ([string](Get-PropertyValue $entry "path"))) -eq $normalized) {
        $match = $true
        break
      }
    }
    if (-not $match) {
      Add-Failure "manifest files missing entry for $normalized"
    }
  }
}

if (-not [string]::IsNullOrWhiteSpace($ZipPath) -and -not [string]::IsNullOrWhiteSpace($PackageDir)) {
  throw "Use either -ZipPath or -PackageDir, not both."
}

Write-Host "RelayCheck package verification"
Write-Host "Root: $Root"

$extractedDir = ""
$packageRoot = ""
if (-not [string]::IsNullOrWhiteSpace($ZipPath)) {
  $zipFull = [System.IO.Path]::GetFullPath($ZipPath)
  if (-not (Test-Path -LiteralPath $zipFull -PathType Leaf)) {
    throw "Zip not found: $zipFull"
  }
  Write-Step "Zip sidecar"
  Test-ZipSidecar $zipFull
  Write-Step "Extract package"
  $extractedDir = Expand-ZipForValidation $zipFull
  $packageRoot = $extractedDir
} elseif (-not [string]::IsNullOrWhiteSpace($PackageDir)) {
  $packageRoot = (Resolve-Path -LiteralPath $PackageDir).Path
} elseif ((Test-Path -LiteralPath (Join-Path $Root "manifest.json") -PathType Leaf) -and (Test-Path -LiteralPath (Join-Path $Root "checksums.sha256") -PathType Leaf)) {
  $packageRoot = $Root
} else {
  $zipFull = Get-LatestZip
  Write-Step "Zip sidecar"
  Test-ZipSidecar $zipFull
  Write-Step "Extract package"
  $extractedDir = Expand-ZipForValidation $zipFull
  $packageRoot = $extractedDir
}

Write-Step "Required files"
Write-Host "Package root: $packageRoot"
$requiredFiles = @(
  "relaycheck.exe",
  "README.md",
  "docs/LAUNCH_READINESS.md",
  "docs/OPERATOR_RUNBOOK.md",
  "docs/OPERATOR_ACCEPTANCE_RECORD.md",
  "scripts/operator-acceptance.ps1",
  "scripts/operator-launch.ps1",
  "scripts/verify-package.ps1",
  "manifest.json",
  "checksums.sha256"
)
foreach ($requiredFile in $requiredFiles) {
  Assert-RequiredFile $packageRoot $requiredFile
}

Write-Step "Manifest"
Test-Manifest $packageRoot $requiredFiles

Write-Step "Checksums"
$checksumRequiredFiles = $requiredFiles | Where-Object { $_ -ne "checksums.sha256" }
Test-ChecksumFile $packageRoot $checksumRequiredFiles

if (-not [string]::IsNullOrWhiteSpace($extractedDir) -and -not $KeepExtracted) {
  Remove-RootItem $extractedDir
}

if ($failures.Count -gt 0) {
  Write-Host ""
  foreach ($failure in $failures) {
    Write-Host " - $failure"
  }
  throw "$($failures.Count) package verification check(s) failed."
}

Write-Host ""
Write-Host "Package verification passed."
