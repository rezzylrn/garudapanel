package panel

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
)

// Routes memasang semua rute panel (REST + WS daemon + static embed).
func Routes(s *Server) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(limitBody)

	// router frame rpc file manager (rpc:res + file:read-chunk dari daemon)
	daemonhub.SetRPCFrameHandler(func(f daemonhub.Frame) {
		switch f.Type {
		case "rpc:res":
			resolveRPCWait(f.Payload)
		case "file:read-chunk":
			deliverReadChunk(f.Payload)
		}
	})

	// daemon join + reverse WS (auth via join/node token, bukan session)
	r.Post("/api/daemon/join", s.handleDaemonJoin)
	r.Get("/api/daemon/ws", daemonhub.WSHandler(s.DB, s.Hub))

	// server user routes: power + console WS + token console
	installServerRoutes(s, r)

	// auth
	r.Post("/api/auth/login", s.handleLogin)
	r.Post("/api/auth/logout", wrapH(s.Sessions.RequireAuth(http.HandlerFunc(s.handleLogout))))
	r.Get("/api/auth/me", wrapH(s.Sessions.RequireAuth(http.HandlerFunc(s.handleMe))))

	// admin (auth + is_admin)
	r.Route("/api/admin", func(ar chi.Router) {
		ar.Use(s.Sessions.RequireAuth)
		ar.Use(auth.RequireAdmin)

		ar.Get("/users", s.handleUsersList)
		ar.Post("/users", s.handleUsersCreate)
		ar.Get("/users/{id}", s.handleUsersGet)
		ar.Patch("/users/{id}", s.handleUsersUpdate)
		ar.Delete("/users/{id}", s.handleUsersDelete)

		ar.Get("/locations", s.handleLocationsList)
		ar.Post("/locations", s.handleLocationsCreate)
		ar.Delete("/locations/{id}", s.handleLocationsDelete)

		ar.Get("/nodes", s.handleNodesList)
		ar.Post("/nodes", s.handleNodesCreate)
		ar.Get("/nodes/{id}", s.handleNodesGet)
		ar.Patch("/nodes/{id}", s.handleNodesUpdate)
		ar.Delete("/nodes/{id}", s.handleNodesDelete)
		ar.Post("/nodes/{id}/revoke-token", s.handleNodesRevokeToken)

		ar.Get("/allocations", s.handleAllocationsList)
		ar.Get("/allocations/{id}", s.handleAllocationsList) // ?node_id filter via path
		ar.Post("/allocations", s.handleAllocationsCreate)

		ar.Get("/nests", s.handleNestsList)
		ar.Post("/nests", s.handleNestsCreate)

		ar.Get("/eggs", s.handleEggsList)
		ar.Post("/eggs", s.handleEggsCreate)
		ar.Get("/eggs/{id}", s.handleEggsGet)
		ar.Patch("/eggs/{id}", s.handleEggsUpdate)
		ar.Delete("/eggs/{id}", s.handleEggsDelete)
		ar.Post("/eggs/{id}/variables", s.handleEggVariablesCreate)
		ar.Patch("/eggs/{id}/variables/{vid}", s.handleEggVariableUpdate)
		ar.Delete("/eggs/{id}/variables/{vid}", s.handleEggVariableDelete)

		ar.Get("/servers", s.handleServersList)
		ar.Post("/servers", s.handleServersCreate)
		ar.Get("/servers/{id}", s.handleServersGet)
		ar.Delete("/servers/{id}", s.handleServersDelete)
	})

	// frontend placeholder (fase 4: go:embed build Svelte)
	r.Get("/*", staticHandler())

	return r
}

// installUserRoutes: rute user (owner/admin) — power, console WS, stats, files.
func installServerRoutes(s *Server, r chi.Router) {
	r.Route("/api/servers/{id}", func(sr chi.Router) {
		sr.Use(s.Sessions.RequireAuth)
		sr.Post("/power", s.handleServerPower)
		sr.Post("/console-token", s.handleConsoleToken)
		sr.Get("/ws", s.handleServerWS) // auth via frame pertama (token 60s)

		// file manager (milestone 3)
		sr.Get("/files", s.handleFileList)
		sr.Get("/files/usage", s.handleFileUsage)
		sr.Get("/files/content", s.handleFileRead)
		sr.Put("/files", s.handleFileWrite) // upload (body streaming)
		sr.Post("/files/mkdir", s.handleFileMkdir)
		sr.Post("/files/rename", s.handleFileRename)
		sr.Post("/files/extract", s.handleFileExtract)
		sr.Delete("/files", s.handleFileDelete)

		// egg variables (milestone 3): lihat/ubah variable editable
		sr.Get("/variables", s.handleServerVariablesGet)
		sr.Patch("/variables/{name}", s.handleServerVariableSet)
	})
}

// wrapH: HandlerFunc → http.Handler (helper komposisi middleware chi).
func wrapH(h http.Handler) http.HandlerFunc {
	return h.ServeHTTP
}

// staticHandler serve web/dist (placeholder dulu; nanti go:embed).
func staticHandler() http.HandlerFunc {
	fs := http.FileServer(http.Dir("web/dist"))
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		full := filepath.Join("web/dist", filepath.Clean("/"+path))
		if _, err := os.Stat(full); err != nil {
			// SPA fallback
			http.ServeFile(w, r, filepath.Join("web/dist", "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}
}
