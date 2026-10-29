package daemonhub

import (
	"database/sql"
	"log"
	"regexp"
	"strings"
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

// substituteStartupVars: ganti placeholder {{NAME}} di startup command egg
// dengan nilai variabel server (env KEY=value yang sama dikirim ke daemon).
// Placeholder yang tidak dikenal dibiarkan apa adanya. ${VAR} TIDAK disentuh
// — diekspansi shell dari env saat proses jalan.
func substituteStartupVars(cmd string, env []string) string {
	if cmd == "" {
		return "echo 'garuda: egg has no startup command'"
	}
	vars := map[string]string{}
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 {
			vars[kv[:i]] = kv[i+1:]
		}
	}
	return varPlaceholderRe.ReplaceAllStringFunc(cmd, func(m string) string {
		name := m[2 : len(m)-2]
		if v, ok := vars[name]; ok {
			return v
		}
		return m
	})
}

var varPlaceholderRe = regexp.MustCompile(`\{\{[A-Za-z_][A-Za-z0-9_]*\}\}`)
