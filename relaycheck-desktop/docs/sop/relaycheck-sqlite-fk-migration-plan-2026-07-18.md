# SQLite Full-Table Foreign Keys — Design Plan

- **Date:** 2026-07-18
- **Status:** **Design only.** Not implemented. Not applied to `data/relaycheck.db`.
- **Scope:** RelayCheck Desktop local SQLite (`internal/core/db.go` migrate path)
- **Related:**  
  - App cascade delete (live): `docs/sop/relaycheck-site-delete-cascade-2026-07-18.md`  
  - Orphan precheck: `docs/sop/relaycheck-site-orphan-precheck-2026-07-18.md`  
  - Existing rebuild pattern: `ensureChannelSchedulesNullableSiteID` in `internal/core/db.go`

---

## 1. Why this exists

SQLite only enforces FKs that are **declared on the table definition**. Adding a column later via `ALTER TABLE` cannot attach a FK. Historical tables were created without FKs; only `channel_schedules` was later rebuilt with:

```sql
FOREIGN KEY (upstream_site_id) REFERENCES upstream_sites(id) ON DELETE CASCADE
```

Today integrity for site delete is enforced by **application-level cascade** (`sites.Service.DeleteUpstreamSite`) + optional orphan cleanup. DB-level FKs would be a second safety net (defense in depth), not a product feature by themselves.

**Recommendation (default):** keep app cascade as source of truth; ship FK rebuild only if product wants DB-enforced integrity for external tools / multi-writer risk. Cost is high (table rewrite + orphan scrub + dual-path testing).

---

## 2. Current state (code + local sample DB)

### 2.1 Code facts

| Fact | Location |
|---|---|
| DSN opens with `foreign_keys(1)` | `internal/core/app.go` `openAppDB` |
| CREATE TABLE (fresh install) for accounts/logs/balances/pricing | **No** FK clauses in `migrate()` |
| Only declared FK | `channel_schedules.upstream_site_id → upstream_sites(id) ON DELETE CASCADE` |
| Site delete | App TX deletes children then parent (`internal/sites/service.go`) |
| Account delete | `DELETE FROM channel_accounts` only — **no** log/balance child cleanup in app today |
| Proven rebuild pattern | `ensureChannelSchedulesNullableSiteID` (`PRAGMA foreign_keys=OFF` → new table → copy → drop → rename → indexes) |

### 2.2 Local DB snapshot (readonly, 2026-07-18 post site-orphan cleanup)

| Table | Rows | Declared FKs |
|---|---:|---|
| `upstream_sites` | 60 | none |
| `channel_accounts` | 25 | none |
| `checkin_logs` | 515 | none |
| `balance_snapshots` | 17 | none |
| `channel_schedules` | 1 | site CASCADE (nullable site id) |
| `site_pricing_cache` | 31 | none |
| `imported_channels` | 128 | none |

**Integrity probes (local):**

| Check | Count | Blocks which FK? |
|---|---:|---|
| accounts → missing site | 0 | site→accounts |
| logs → missing site | 0 | site→logs |
| balances → missing site | 0 | site→balances |
| pricing → missing site | 0 | site→pricing |
| schedules → missing site | 0 | already FK |
| **logs → missing account** | **28** | **accounts→logs** |
| balances → missing account | 0 | accounts→balances |
| sites → missing imported channel | 0 | optional channel FK |
| logs site vs account.site mismatch | 0 | consistency |

> Any host must re-run these probes before migration. Numbers above are **this machine only**.

---

## 3. Goals / non-goals

### Goals

1. Document the **target FK graph** and ON DELETE/UPDATE actions.
2. Define a **phased, reversible** rebuild migration that matches existing `ensure*Rebuild` style.
3. Gate migration on **precheck = 0 orphans** for every FK being added.
4. Keep app-level site cascade correct under FK ON (no double-delete errors; order-safe).
5. Preserve `channel_schedules.upstream_site_id` **NULL** = global schedule semantics (never force NOT NULL).

### Non-goals (this plan)

- Soft-delete / archive-site product mode
- Cross-DB (Postgres) multi-tenant FKs
- Auto-running rewrite on every launch without version gate
- Changing product delete UX
- Implementing migration code without a second explicit confirm after this doc is accepted

---

## 4. Target FK graph

### 4.1 Phase A — Site subtree (recommended first if ever implemented)

Align DB with already-shipped **site delete** cascade.

| Child | Column | Parent | ON DELETE | ON UPDATE | Notes |
|---|---|---|---|---|---|
| `channel_accounts` | `upstream_site_id` | `upstream_sites(id)` | **CASCADE** | NO ACTION | Column already `NOT NULL` |
| `checkin_logs` | `upstream_site_id` | `upstream_sites(id)` | **CASCADE** | NO ACTION | Also has `account_id` (Phase B) |
| `balance_snapshots` | `upstream_site_id` | `upstream_sites(id)` | **CASCADE** | NO ACTION | |
| `site_pricing_cache` | `site_id` | `upstream_sites(id)` | **CASCADE** | NO ACTION | Naming is `site_id` not `upstream_site_id` |
| `channel_schedules` | `upstream_site_id` | `upstream_sites(id)` | **CASCADE** | NO ACTION | **Already live**; site id nullable |

**App interaction:** `DeleteUpstreamSite` already deletes children first then parent. With CASCADE, either:

- **Keep app deletes** (preferred initially): redundant but explicit counts for API response; parent delete becomes no-op for children already gone; or  
- **Simplify later** to `DELETE FROM upstream_sites WHERE id=?` only and derive counts via `RETURNING`/pre-select — optional follow-up, not required for Phase A.

### 4.2 Phase B — Account subtree

| Child | Column | Parent | ON DELETE | Notes |
|---|---|---|---|---|
| `checkin_logs` | `account_id` | `channel_accounts(id)` | **CASCADE** | Blocked locally by **28** orphan logs |
| `balance_snapshots` | `account_id` | `channel_accounts(id)` | **CASCADE** | Local OK |

**App interaction:** account delete currently does **not** cascade logs/balances. Phase B **changes product semantics** unless app is updated first:

1. Either extend `deleteAccount` TX to delete logs + balances (mirror site delete), **or**
2. Rely solely on FK CASCADE and accept silent child removal (must document + return counts).

**Plan default:** implement app cascade for account delete **before** enabling Phase B FKs (same pattern as sites).

### 4.3 Phase C — Optional weak links (defer)

| Child | Column | Parent | Suggested action | Why defer |
|---|---|---|---|---|
| `upstream_sites` | `channel_id` | `imported_channels(id)` | **SET NULL** | Channel can disappear from local NewAPI sync; site should remain |
| `checkin_logs` | `channel_id` | `imported_channels(id)` | **SET NULL** | Historical denormalized tag |
| `balance_snapshots` | `channel_id` | `imported_channels(id)` | **SET NULL** | Same |
| `imported_channels` | `local_instance_id` | `local_newapi_instances(id)` | **SET NULL** or CASCADE | Instance removal rare; product-owned |

Do **not** CASCADE site when channel is deleted — sites are user-owned inventory.

### 4.4 Explicitly excluded

| Idea | Decision |
|---|---|
| FK from FTS shadow tables | Managed by FTS triggers; leave alone |
| `audit_log` / `app_notifications` entity refs | Free-form JSON/text; no FK |
| `scheduler_runs` | No stable site FK today |
| Composite FK (account_id, site_id) matching parent | Nice-to-have; SQLite painful; enforce in app if needed |

---

## 5. SQLite rebuild recipe (per table)

Pattern already proven on `channel_schedules`:

```
1. Backup DB files (relaycheck.db + wal + shm)
2. PRAGMA foreign_keys=OFF on dedicated connection
3. BEGIN
4. CREATE TABLE <t>_new ( ... full column list + FOREIGN KEY ... )
5. INSERT INTO <t>_new SELECT ... FROM <t>   -- may filter/fix orphans
6. DROP TABLE <t>
7. ALTER TABLE <t>_new RENAME TO <t>
8. Recreate indexes / triggers (FTS if accounts)
9. COMMIT
10. PRAGMA foreign_keys=ON
11. PRAGMA foreign_key_check
12. Smoke app open + site delete test on copy DB
```

### 5.1 Column list discipline

- Rebuild SQL must list **every live column**, including those added via `ensureColumn` (accounts has many `api_key_*` columns).  
- **Source of truth for column set:** `PRAGMA table_info(t)` at migration authoring time + codegen/test that fails if migrate CREATE and live pragma diverge.  
- Fresh-install `CREATE TABLE IF NOT EXISTS` in `migrate()` must be updated to the same FK-bearing definition so new installs and rebuilt installs converge.

### 5.2 Order of Phase A rebuilds

Safe order (children first is irrelevant for rewrite; parent must exist before FK check on copy):

1. Ensure parent `upstream_sites` intact  
2. Rebuild `channel_accounts` (site FK only in Phase A)  
3. Rebuild `checkin_logs` (site FK only in Phase A; account FK later)  
4. Rebuild `balance_snapshots` (site FK)  
5. Rebuild `site_pricing_cache` (site FK)  
6. Confirm `channel_schedules` already correct  
7. `PRAGMA foreign_key_check`

Phase B rebuilds `checkin_logs` / `balance_snapshots` again **or** combines both FKs in one rebuild if Phase A+B ship together (preferred if both approved — one rewrite per table).

### 5.3 Version gate

Add a settings/meta key e.g. `schema_fk_phase = 0|1|2` in `system_settings` so rebuild is idempotent:

- `0` = pre-FK (current production shape for accounts/logs/…)  
- `1` = Phase A applied  
- `2` = Phase B applied  

Never re-run rebuild when gate already satisfied.

---

## 6. Pre-migration gates (hard fail)

Migration **must abort** (no partial rename) if any gate fails:

### Gate G0 — Backup

- Operator backup exists and is restorable (Settings backup or file copy of `db*`).  
- Prefer run against **copied** DB first in CI/dev.

### Gate G1 — Site orphans (Phase A)

Reuse / extend `scripts/sql/precheck-site-orphans.sql`:

- All five site-join tables `orphan_rows = 0`.

### Gate G2 — Account orphans (Phase B)

```sql
-- checkin_logs missing parent account
SELECT COUNT(*) FROM checkin_logs l
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = l.account_id);

-- balance_snapshots missing parent account
SELECT COUNT(*) FROM balance_snapshots b
WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id = b.account_id);
```

Local sample: logs **28** → **must clean or reassign before Phase B**.

### Gate G3 — Empty / invalid keys

- Phase A: no empty `upstream_site_id` on accounts/logs/balances; no empty `site_id` on pricing.  
- Phase B: no empty `account_id` on logs/balances (already NOT NULL in schema).

### Gate G4 — foreign_key_check

After rebuild:

```sql
PRAGMA foreign_key_check;
PRAGMA foreign_keys;  -- must be 1 on app DSN
```

Any row → fail release.

### Gate G5 — Behavioral tests

- `go test ./internal/sites -run DeleteUpstream`  
- New: delete site with children under FK ON (integration on temp DB)  
- New: delete account with logs under Phase B  
- Fresh migrate from empty DB creates FK-bearing tables  
- Upgrade path: old DB fixture → migrate → pragma foreign_key_list matches expected

---

## 7. Orphan policy before Phase B

For the **28** local `checkin_logs` with missing `account_id` parents:

| Option | Effect | When to use |
|---|---|---|
| **B1 Delete orphans** | Lose historical log lines for deleted accounts | Default if logs are operational telemetry only |
| **B2 Reassign** | Impossible without knowing original account | Don't |
| **B3 SET NULL account_id** | Requires nullable column + product meaning | Reject — breaks account-scoped history queries |
| **B4 Skip Phase B** | Keep site FKs only | Valid product choice |

**Plan default:** B1 after backup + dry-run list (mirror checkin site-orphan cleanup SOP), then Phase B.

Add tooling (design only here):

- `scripts/sql/precheck-account-orphans.sql`  
- `scripts/sql/cleanup-checkin-log-account-orphans.dry-run.sql`  
- Mutating cleanup only after explicit confirm (same bar as site orphans)

---

## 8. Interaction matrix

| Operation today | Without full FK | After Phase A | After Phase B |
|---|---|---|---|
| Delete site (app TX) | App deletes children | App deletes children; CASCADE backup if app misses a table | Same |
| Delete site (raw SQL) | Orphans | CASCADE cleans site children | Same + account children go with accounts |
| Delete account (app) | Leaves logs | Leaves logs | CASCADE deletes logs/balances **or** app must delete first |
| Insert log with bad site id | Succeeds | **Fails** FK | Fails |
| Insert log with bad account id | Succeeds | Succeeds (Phase A only) | **Fails** |
| Global schedule `upstream_site_id NULL` | OK | OK | OK |

---

## 9. Risk register

| Risk | Severity | Mitigation |
|---|---|---|
| Rebuild misses a `ensureColumn` field → data loss | **Critical** | Generate column list from pragma; golden test |
| Long lock on large `checkin_logs` | Medium | Offline migrate; WAL checkpoint; progress log |
| FTS/account search triggers break on accounts rebuild | High | Recreate FTS in same TX (`ensureAccountSearchFTS`) |
| Double CASCADE + app delete confuses counts | Low | Pre-select counts before delete; keep app order |
| Hosts still have orphans → migrate crashes mid-flight | High | Gates G1–G2; abort before DROP |
| Operators expect soft-delete | Product | Out of scope; document irreversible CASCADE |
| Partial Phase A without updating fresh CREATE | Medium | Single source DDL snippets shared by migrate + rebuild |

---

## 10. Implementation sketch (for a future PR — not this change)

Suggested code shape (illustrative):

```text
internal/core/db.go
  migrate()
    ... existing ensureColumn ...
    if err := a.ensureSiteSubtreeForeignKeys(ctx); err != nil { return err } // Phase A gate
    // later: ensureAccountSubtreeForeignKeys

  ensureSiteSubtreeForeignKeys
    if schema_fk_phase >= 1 { return nil }
    if err := a.assertNoSiteOrphans(ctx); err != nil { return err }
    rebuild channel_accounts / checkin_logs / balance_snapshots / site_pricing_cache
    set schema_fk_phase = 1

internal/core/db_fk_test.go
  temp DB old-shape fixture → migrate → PRAGMA foreign_key_list
  insert orphan should fail
  delete site cascade behavior
```

**Ship order for PR series:**

1. Docs (this file) + account-orphan precheck SQL  
2. Account-orphan cleanup tooling (dry-run) — optional execute  
3. Phase A migration code + tests on fixtures only  
4. Operator confirm → run on real data hosts  
5. Account delete app cascade  
6. Phase B migration  

---

## 11. Decision record (proposed)

| Decision | Choice | Rationale |
|---|---|---|
| D1 Need full FK now? | **No (default)** | App site cascade + orphan=0 for sites already closes the original bug |
| D2 If yes, first ship | **Phase A only** | Matches existing product delete; low semantic surprise |
| D3 Phase B | After account-orphan cleanup + app account cascade | Avoid silent log loss / migrate fail on 28 orphans |
| D4 Phase C channel FKs | Defer | SET NULL semantics need product copy |
| D5 ON DELETE for site children | CASCADE | Matches app + schedules precedent |
| D6 Global schedule | Keep NULL site id | Already migrated once |

**To approve implementation:** operator replies with explicit scope, e.g.

- `批准 FK Phase A 实现` — code + fixture tests only, no auto-touch of personal data DB beyond migrate-on-next-launch  
- `批准 FK Phase A+B` — includes account orphan cleanup policy B1  
- `维持仅应用级级联` — close this track; keep doc as ADR

---

## 12. Acceptance criteria (when implementation is approved)

- [ ] Fresh DB: `PRAGMA foreign_key_list` matches Phase target  
- [ ] Upgraded fixture DB: same  
- [ ] `PRAGMA foreign_key_check` empty  
- [ ] Site delete tests green with FK ON  
- [ ] Account delete defined for Phase B  
- [ ] Precheck scripts in CI or `go test` integration  
- [ ] HANDOFF updated; backup SOP linked  
- [ ] No silent migrate on dirty orphan DB (hard error with repair instructions)

---

## 13. Operator commands (readonly today)

```powershell
cd E:\zidqiandao\relaycheck-desktop
# site orphans
powershell -NoProfile -File .\scripts\precheck-site-orphans.ps1

# account orphans (ad-hoc until script lands)
python -c "import sqlite3,os;c=sqlite3.connect(os.path.abspath('data/relaycheck.db'));print('log_orphan_acc',c.execute('SELECT COUNT(*) FROM checkin_logs l WHERE NOT EXISTS (SELECT 1 FROM channel_accounts a WHERE a.id=l.account_id)').fetchone()[0])"
```

---

## 14. Changelog

| Date | Note |
|---|---|
| 2026-07-18 | Initial design from live schema + post site-orphan cleanup integrity sample; **not implemented** |
