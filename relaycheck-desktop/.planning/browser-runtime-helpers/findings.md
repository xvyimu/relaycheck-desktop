# Findings: Browser Runtime Helpers

## Source Findings

- `accounts.go` still contains browser runtime details:
  - `estimateCookieExpiry`
  - `cdpCookie`
  - `cdpResponse`
  - `readChromeSession`
  - `findPageWebSocket`
  - `buildCookieHeader`
  - `freeDebugPort`
  - `findChrome`
- These helpers are primarily used by `BrowserLoginService`.
- Existing tests cover:
  - `TestEstimateCookieExpiry`
  - `TestBuildCookieHeader`
- Missing coverage:
  - `freeDebugPort` skips used ports and returns an available port.

## Decisions

- Move browser runtime helpers into `browser_runtime.go` inside `package core`.
- Keep helper names and signatures unchanged.
- Add one focused `freeDebugPort` test before moving code.
- Leave `clearAccountSession` in `accounts.go` for this slice; it is a handler/service extraction candidate for a later pass.
