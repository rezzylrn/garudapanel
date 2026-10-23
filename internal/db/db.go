// Package db menyediakan koneksi SQLite (WAL) + migrasi embedded.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open membuka (dan membuat bila perlu) database SQLite di path,
// mengaktifkan WAL + foreign_keys, lalu menjalankan semua migrasi.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// SQLite WAL: reader konkuren didukung; pool diset ke 8 agar tidak deadlock
	// saat query bertingkat/konkuren sambil tetap aman untuk single writer SQLite.
	sqlDB.SetMaxOpenConns(8)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := Migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

// Migrate menjalankan migrasi embedded (golang-migrate-compatible file naming:
// 000001_name.up.sql / 000001_name.down.sql) tanpa menutup koneksi yang diberikan.
func Migrate(sqlDB *sql.DB) error {
	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, dirty INTEGER NOT NULL DEFAULT 0)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	ups := map[string]string{} // version -> sql
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version := strings.SplitN(name, "_", 2)[0]
		b, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		ups[version] = string(b)
	}
	versions := make([]string, 0, len(ups))
	for v := range ups {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	var current int64
	if err := sqlDB.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read current version: %w", err)
	}
	for _, v := range versions {
		var ver int
		if _, err := fmt.Sscanf(v, "%d", &ver); err != nil {
			return fmt.Errorf("bad migration version %q", v)
		}
		if ver <= int(current) {
			continue
		}
		if _, err := sqlDB.Exec(ups[v]); err != nil {
			return fmt.Errorf("apply migration %s: %w", v, err)
		}
		if _, err := sqlDB.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, ver); err != nil {
			return fmt.Errorf("record version %d: %w", ver, err)
		}
	}
	return nil
}
