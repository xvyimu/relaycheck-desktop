package core

import (
	"reflect"
	"testing"
)

func TestMigrateCreatesPerformanceIndexes(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	expected := map[string][]string{
		"imported_channels": {
			"idx_imported_channels_source_status_updated",
			"idx_imported_channels_kind_updated",
		},
		"upstream_sites": {
			"idx_upstream_sites_kind_updated",
			"idx_upstream_sites_updated",
		},
		"channel_accounts": {
			"idx_channel_accounts_updated",
			"idx_channel_accounts_key_check",
		},
		"checkin_logs": {
			"idx_checkin_logs_account_started",
		},
		"balance_snapshots": {
			"idx_balance_snapshots_account_created",
			"idx_balance_snapshots_site_created",
		},
		"app_notifications": {
			"idx_app_notifications_read_created",
		},
	}

	for table, names := range expected {
		indexes := loadIndexNames(t, app, table)
		for _, name := range names {
			if !indexes[name] {
				t.Fatalf("expected index %s on table %s, got indexes %#v", name, table, indexes)
			}
		}
	}

	for index, want := range map[string][]string{
		"idx_checkin_logs_account_started_id":      {"account_id", "started_at", "id"},
		"idx_balance_snapshots_account_created_id": {"account_id", "created_at", "id"},
	} {
		if got := loadIndexColumns(t, app, index); !reflect.DeepEqual(got, want) {
			t.Fatalf("index %s columns = %v, want %v", index, got, want)
		}
	}
}

func loadIndexColumns(t *testing.T, app *App, index string) []string {
	t.Helper()
	rows, err := app.db.Query("PRAGMA index_info(" + index + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var seq, cid int
		var name string
		if err := rows.Scan(&seq, &cid, &name); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return columns
}

func loadIndexNames(t *testing.T, app *App, table string) map[string]bool {
	t.Helper()

	rows, err := app.db.Query("PRAGMA index_list(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	indexes := map[string]bool{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatal(err)
		}
		indexes[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return indexes
}
