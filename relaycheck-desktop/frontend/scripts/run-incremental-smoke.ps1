$ErrorActionPreference = "Stop"

$frontendDir = Split-Path -Parent $PSScriptRoot
$viteOutput = Join-Path $env:TEMP ("relaycheck-vite-{0}.out.log" -f [guid]::NewGuid())
$viteError = Join-Path $env:TEMP ("relaycheck-vite-{0}.err.log" -f [guid]::NewGuid())
$vite = Start-Process -FilePath "npm.cmd" `
  -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1") `
  -WorkingDirectory $frontendDir `
  -PassThru `
  -RedirectStandardOutput $viteOutput `
  -RedirectStandardError $viteError `
  -WindowStyle Hidden

try {
  $ready = $false
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:5173/" -TimeoutSec 2
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) {
        $ready = $true
        break
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  if (-not $ready) {
    $details = @()
    if (Test-Path -LiteralPath $viteOutput) {
      $details += Get-Content -Raw -LiteralPath $viteOutput
    }
    if (Test-Path -LiteralPath $viteError) {
      $details += Get-Content -Raw -LiteralPath $viteError
    }
    throw "Vite did not become ready at http://127.0.0.1:5173/: $($details -join [Environment]::NewLine)"
  }

  & npm.cmd run smoke:incremental
  if ($LASTEXITCODE -ne 0) {
    throw "Incremental browser smoke failed with exit code $LASTEXITCODE"
  }
} finally {
  if (-not $vite.HasExited) {
    & taskkill.exe /PID $vite.Id /T /F | Out-Null
  }
  Remove-Item -LiteralPath $viteOutput, $viteError -Force -ErrorAction SilentlyContinue
}
