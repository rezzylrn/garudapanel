// Command garuda: satu binary untuk panel dan daemon (SPEC §1).
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"garudapanel/internal/auth"
	"garudapanel/internal/daemon"
	"garudapanel/internal/daemonhub"
	"garudapanel/internal/db"
	"garudapanel/internal/panel"
)

const usage = `GarudaPanel — panel manajemen game server.

Usage:
  garuda panel  [--addr :8080] [--db garuda-panel.db]
  garuda daemon --panel http://host:8080 --join TOKEN
  garuda admin reset-password [--db garuda-panel.db]

Commands:
  panel   jalankan panel HTTP (API + UI)
  daemon  jalankan daemon node (wajib --join untuk registrasi pertama)
  admin   manajemen user admin (subcommand: reset-password)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "panel":
		cmdPanel(os.Args[2:])
	case "daemon":
		cmdDaemon(os.Args[2:])
	case "admin":
		cmdAdmin(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "garuda: unknown command %q\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func cmdPanel(args []string) {
	fs := flag.NewFlagSet("panel", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "listen address")
	dbPath := fs.String("db", "garuda-panel.db", "sqlite database path")
	bootstrapPass := fs.String("bootstrap-admin", "", "password admin awal (kalau kosong: dibuat acak lalu di-print)")
	maxUploadMB := fs.Int64("max-upload-mb", 100, "batas ukuran upload file dalam MB (default: 100)")
	_ = fs.Parse(args)

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		fatal("db: %v", err)
	}
	defer sqlDB.Close()

	if err := bootstrapAdmin(sqlDB, *bootstrapPass); err != nil {
		fatal("bootstrap admin: %v", err)
	}

	srv := &panel.Server{
		Sessions:       auth.NewSessionStore(sqlDB),
		DB:             sqlDB,
		Hub:            daemonhub.NewHub(),
		MaxUploadBytes: *maxUploadMB * 1024 * 1024,
	}
	// router frame daemon → browser console (SPEC §4)
	srv.Hub.SetOnFrame(func(nodeID int64, f daemonhub.Frame) {
		panel.ConsoleRouter(f)
	})
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           panel.Routes(srv),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("panel listening on %s (db=%s)", *addr, *dbPath)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal("serve: %v", err)
	}
}

func cmdDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	join := fs.String("join", "", "join token sekali pakai dari panel (respons POST /api/admin/nodes)")
	panelURL := fs.String("panel", "http://127.0.0.1:8080", "URL panel")
	dataDir := fs.String("data", "/var/lib/garudapanel", "dir data daemon (lokasi node-token)")
	_ = fs.Parse(args)

	opts := daemon.JoinOptions{
		PanelURL:  *panelURL,
		JoinToken: *join,
		StorePath: *dataDir + "/node-token",
		DataDir:   *dataDir + "/volumes",
	}
	if *join == "" {
		// tanpa --join: pakai node token tersimpan dari sesi join sebelumnya
		tok, err := daemon.LoadNodeToken(opts.StorePath)
		if err != nil {
			fatal("no --join given and no saved node token at %s (%v)", opts.StorePath, err)
		}
		opts.JoinToken = "" // sudah punya token long-lived
		opts.ResumeToken = tok
	}
	if err := daemon.Join(opts); err != nil {
		fatal("%v", err)
	}
}

func cmdAdmin(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: garuda admin <reset-password> [options]\n")
		os.Exit(2)
	}
	switch args[0] {
	case "reset-password":
		cmdAdminResetPassword(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "garuda admin: unknown command %q\n", args[0])
		os.Exit(1)
	}
}

func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

func cmdAdminResetPassword(args []string) {
	fs := flag.NewFlagSet("admin reset-password", flag.ExitOnError)
	dbPath := fs.String("db", "garuda-panel.db", "sqlite database path")
	_ = fs.Parse(args)

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		fatal("db: %v", err)
	}
	defer sqlDB.Close()

	password, err := generateRandomPassword(16)
	if err != nil {
		fatal("generate password: %v", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fatal("hash password: %v", err)
	}

	var exists int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'admin'`).Scan(&exists)
	if err != nil {
		fatal("query user: %v", err)
	}

	if exists > 0 {
		_, err = sqlDB.Exec(`UPDATE users SET password_hash = ?, is_admin = 1 WHERE username = 'admin'`, hash)
		if err != nil {
			fatal("update admin: %v", err)
		}
	} else {
		_, err = sqlDB.Exec(`INSERT INTO users (username, email, password_hash, is_admin)
			VALUES ('admin', 'admin@localhost', ?, 1)`, hash)
		if err != nil {
			fatal("create admin: %v", err)
		}
	}

	fmt.Printf("ADMIN RESET -> username: admin password: %s\n", password)
}

// bootstrapAdmin buat user admin pertama kalau belum ada user sama sekali.
func bootstrapAdmin(sqlDB *sql.DB, password string) error {
	var cnt int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	if password == "" {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		password = "gp-" + base64.RawURLEncoding.EncodeToString(b)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (username, email, password_hash, is_admin)
		VALUES ('admin', 'admin@localhost', ?, 1)`, hash); err != nil {
		return err
	}
	fmt.Printf("BOOTSTRAP ADMIN dibuat -> username: admin  password: %s\n(simpan sekarang; pesan ini hanya muncul sekali)\n", password)
	return nil
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "garuda: "+f+"\n", a...)
	os.Exit(1)
}
