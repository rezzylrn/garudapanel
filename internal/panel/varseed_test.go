package panel

import (
	"database/sql"
	"testing"
)

// seed helpers untuk variables_test.go (DB in-memory).

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func lastID(t *testing.T, db *sql.DB, table string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM ` + table + ` ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("lastID %s: %v", table, err)
	}
	return id
}

func seedVar(t *testing.T, db *sql.DB, eggID int64, name, def string, viewable, editable bool) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO egg_variables (egg_id, name, default_value, user_viewable, user_editable, description)
		VALUES (?,?,?,?,?,'')`, eggID, name, def, boolInt(viewable), boolInt(editable)); err != nil {
		t.Fatalf("seedVar %s: %v", name, err)
	}
}
