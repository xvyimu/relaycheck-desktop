package core

import (
	"context"
	"strings"
	"testing"
)

func TestEnsureColumnRejectsInvalidIdentifiers(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	ctx := context.Background()

	cases := []struct {
		name       string
		table      string
		column     string
		columnType string
		wantSubstr string
	}{
		{"table semicolon", "channel_accounts; DROP TABLE", "x", "TEXT", "invalid table identifier"},
		{"column space", "channel_accounts", "bad name", "TEXT", "invalid column identifier"},
		{"column quote", "channel_accounts", "bad'name", "TEXT", "invalid column identifier"},
		{"type semicolon", "channel_accounts", "new_col", "TEXT; DROP TABLE", "forbidden substring"},
		{"type comment", "channel_accounts", "new_col", "TEXT--", "forbidden substring"},
		{"type block", "channel_accounts", "new_col", "TEXT /* */", "forbidden substring"},
		{"type invalid char", "channel_accounts", "new_col", "TEXT@", "invalid column type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := app.ensureColumn(ctx, tc.table, tc.column, tc.columnType)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantSubstr, err.Error())
			}
		})
	}
}

func TestEnsureColumnAcceptsValidDefaultDeclarations(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	ctx := context.Background()

	if _, err := app.db.ExecContext(ctx, `CREATE TABLE ensure_column_fixture (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		column     string
		columnType string
	}{
		{"plain text", "col_text", "TEXT"},
		{"integer not null default", "col_int", "INTEGER NOT NULL DEFAULT 0"},
		{"text default string", "col_default", "TEXT NOT NULL DEFAULT 'active'"},
		{"text default empty array", "col_array", "TEXT NOT NULL DEFAULT '[]'"},
		{"text default empty string", "col_empty", "TEXT NOT NULL DEFAULT ''"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := app.ensureColumn(ctx, "ensure_column_fixture", tc.column, tc.columnType); err != nil {
				t.Fatalf("ensureColumn returned error: %v", err)
			}
		})
	}
}