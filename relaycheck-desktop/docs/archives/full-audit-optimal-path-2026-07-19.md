# RelayCheck Desktop — Full Audit & Optimal Path (2026-07-19)

**Status:** executed (code track still ENDED for product feature work)  
**HEAD at audit:** `c8764d8` / closeout pin `0d400d4` / feature tip `42f21c8`  
**Path:** `E:\zidqiandao\relaycheck-desktop`

## Decision (optimal path from HANDOFF + closeout + audit)

Do **not** reopen Phase C, Authenticode, or multi-host RUM without materials/authorization.

Do the highest-value, unconditional work allowed by docs:

1. Confirm security residual claims against live code  
2. Land remaining safe hardening that needs no secrets  
3. Verify release toolchain path with Go **1.26.5**  
4. Archive this audit so future agents do not re-litigate closed items

## Gate snapshot (audit host)

| Gate | Result |
| --- | --- |
| `go test -mod=vendor ./internal/...` | pass |
| `go test -cover ./internal/core` | **60.1%** (≥55% floor) |
| `go vet -mod=vendor ./ ./internal/...` | pass |
| `go build -mod=vendor .` | pass |
| frontend `tsc -b` | pass |
| frontend vitest | **69 files / 405 tests** pass |
| `npm audit --audit-level=moderate` | 0 vulns |
| Local DB | `schema.fk_phase="2"`, accounts 25, logs 487, `foreign_key_check` empty |
| Host Go install | 1.26.4 on PATH; **GOTOOLCHAIN=go1.26.5** resolves |
| `dist/relaycheck.exe` | NotSigned (expected hard-block without PFX) |

## Security residual triage

| Finding | Audit note | Action |
| --- | --- | --- |
| 5xx `err.Error()` sites | **Already neutralized at boundary**: `writeError` rewrites any ≥500 client message to `服务暂时不可用，请稍后重试。` while logging the original server-side (`internal/core/http.go`) | No bulk rewrite required for confidentiality |
| Proxy status full `url` | Public `NetworkProxyStatus` still JSON-emitted `url` + `urlMasked` | **Fixed this pass** — `json:"-"` on `URL`; clients keep `urlMasked`; edit form still uses `system_settings` `network.proxy` |
| Chrome CDP cookies | Product capability on loopback | Documented; no code change |
| Default no session token | By design for trusted single-user | Optional `RELAYCHECK_REQUIRE_TOKEN=1` |
| Unsigned binary / multi-host RUM / Phase C | External materials or explicit auth | Leave closed |

## Code changes this pass

- `internal/core/network.go` — public proxy status omits full URL  
- `internal/core/network_test.go` — marshal + `/api/system/status` leak guards  
- `frontend/src/types/index.ts` — `NetworkProxyStatus.url` optional  
- `frontend/src/components/settings/__tests__/settings-split.test.ts` — fixture without public `url`

Settings edit path unchanged: full proxy URL remains in `NetworkProxyConfig` / `system_settings`.

## Still external (do not invent)

1. Code Signing PFX + password → `scripts/sign-release.ps1`  
2. Representative extra host / large DB RUM  
3. Explicit product phrase: **批准 Phase C**

## Operator notes for desktop-only use

- Prefer `GOTOOLCHAIN=go1.26.5` (or install 1.26.5) before `scripts/verify-release.ps1`  
- Back up `data/relaycheck.db*` before upgrading binaries (auto FK migrate)  
- Shared host: set `RELAYCHECK_REQUIRE_TOKEN=1`  
- Never delete `data/`; never self-sign “production” certs

## Read order

1. `HANDOFF.md`  
2. `docs/archives/PROJECT-CLOSEOUT-2026-07-19.md`  
3. This file  
4. `docs/deploy/code-signing-readiness-2026-07-18.md` (if packaging for others)
