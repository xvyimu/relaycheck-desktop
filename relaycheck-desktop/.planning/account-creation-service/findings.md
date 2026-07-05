# Findings: Account Creation Service

## Source Findings

- `accounts.go` still owns account creation execution:
  - `createAccount`
  - `ensureManualAccountSite`
- Existing helper coverage:
  - `TestInferAccountAuthType`
  - `TestDefaultAccountDisplayName`
- Existing manual site metadata coverage:
  - `TestEnsureManualAccountSiteStoresManualLoginMetadata`
  - `TestUpdateAccountSiteMetadataMarksLoginURLManual`
- `AccountSiteUpdateService` depends on `App.ensureManualAccountSite`, so that method must remain as a compatibility wrapper.

## Decisions

- Extract `AccountCreationService` inside `package core`.
- Keep `createAccount` as a thin HTTP boundary.
- Keep `ensureManualAccountSite` as a thin wrapper delegating to the service.
- Inject `detectUpstream` into the service so tests can cover manual site creation without network calls.
- Preserve existing response and error behavior at the handler layer.

## Notes

- Historical mojibake exists in nearby user-facing messages. This slice preserves existing text bytes instead of broadening scope.
