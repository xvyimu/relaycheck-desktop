# Findings: Account Site Update Service

## Source Findings

- `accounts.go` still contains account site update functions:
  - `resolveAccountSiteUpdate`
  - `updateSharedAccountSite`
  - `updateAccountSiteAddress`
  - `updateAccountSiteMetadata`
- Existing test coverage:
  - `TestUpdateAccountSiteMetadataMarksLoginURLManual` covers manual login metadata.
- Missing coverage:
  - shared-scope update that moves all accounts from the current site to an existing site with the requested base URL.
  - current-scope creation/selection behavior.

## Decisions

- Extract into `AccountSiteUpdateService` inside `package core`.
- Keep `App` wrappers so existing tests and update handler do not need broad changes.
- Add a focused shared-scope service test before implementation.

## Notes

- A Windows `rg` invocation with `internal\core\*_test.go` failed because PowerShell passed the glob literally; use `rg ... internal\core` or `rg --files` instead.

