[CmdletBinding()]
param(
  [switch]$SkipFrontend
)
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Frontend = Join-Path $Root "frontend"
if (-not $SkipFrontend) {
  Push-Location $Frontend
  try {
    if (-not (Test-Path "node_modules")) { npm ci }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
  } finally { Pop-Location }
}
$version = "v1.1.0"
$routes = Get-Content (Join-Path $Root "internal\core\routes.go") -Raw
$m = [regex]::Match($routes, 'productVersion\s*=\s*"([^"]+)"')
if ($m.Success) { $version = $m.Groups[1].Value }
$commit = "local"
try { $commit = [string](& git -C $Root rev-parse --short=12 HEAD) } catch {}
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-H=windowsgui -s -w -X=relaycheck-desktop/internal/core.productVersion=$version -X=relaycheck-desktop/internal/core.buildTime=$buildTime -X=relaycheck-desktop/internal/core.gitCommit=$commit"
New-Item -ItemType Directory -Force (Join-Path $Root "dist") | Out-Null
Push-Location $Root
try {
  go build -mod=vendor -ldflags=$ldflags -o "dist\relaycheck.exe" .
  if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally { Pop-Location }
Write-Host "Built dist\relaycheck.exe ($version $commit)"
