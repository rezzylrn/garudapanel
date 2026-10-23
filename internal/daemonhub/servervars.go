package daemonhub

import (
	"database/sql"
	"log"
)

// serverVariableEnv: env KEY=value dari egg variables milik egg server ini.
// Nilai = server_variables.value bila diset, else default_value egg.
// Nama variable di-uppercase + non-aman diganti '_' (konvensi env).
func serverVariableEnv(db *sql.DB, serverUUID string) []string {
	rows, err := db.Query(`SELECT v.name, COALESCE(sv.value, v.default_value)
		FROM egg_variables v
		JOIN servers s ON s.egg_id = v.egg_id
		LEFT JOIN server_variables sv ON sv.variable_id = v.id AND sv.server_id = s.id
		WHERE s.uuid = ?`, serverUUID)
	if err != nil {
		log.Printf("serverVariableEnv: %v", err)
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name, val string
		if err := rows.Scan(&name, &val); err != nil {
			continue
		}
		if name == "" || len(name) > 64 {
			continue
		}
		out = append(out, sanitizeEnvName(name)+"="+val)
	}
	return out
}

// sanitizeEnvName: nama env aman — uppercase, non-aman jadi '_'.
func sanitizeEnvName(n string) string {
	out := make([]rune, 0, len(n))
	for _, ch := range n {
		switch {
		case ch >= 'a' && ch <= 'z':
			out = append(out, ch)
		case ch >= 'A' && ch <= 'Z':
			out = append(out, ch)
		case ch >= '0' && ch <= '9':
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
