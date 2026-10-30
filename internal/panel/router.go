package panel

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
	"garudapanel/web"
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
		ar.Post("/nodes/{id}/join-token", s.handleNodesJoinToken)

		ar.Get("/allocations", s.handleAllocationsList)
		ar.Get("/allocations/{id}", s.handleAllocationsList) // ?node_id filter via path
		ar.Post("/allocations", s.handleAllocationsCreate)

		ar.Get("/nests", s.handleNestsList)
		ar.Post("/nests", s.handleNestsCreate)

		ar.Get("/eggs", s.handleEggsList)
		ar.Post("/eggs", s.handleEggsCreate)
		ar.Post("/eggs/import", s.handleEggsImport)
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

	// Unknown /api/* routes must return JSON 404, not the SPA fallback
	// (covers non-GET methods; GET is handled inside staticHandler).
	r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		staticHandler().ServeHTTP(w, r)
	}))

	// frontend placeholder (fase 4: go:embed build Svelte)
	r.Get("/*", staticHandler())

	return r
}

// installUserRoutes: rute user (owner/admin) — power, console WS, stats, files.
func installServerRoutes(s *Server, r chi.Router) {
	r.Route("/api/servers/{id}", func(sr chi.Router) {
		// /ws TANPA RequireAuth: browser tidak bisa mengirim header Bearer
		// saat handshake WebSocket. Auth dilakukan via frame pertama
		// {t:"auth", token} — console token sekali pakai 60 detik.
		sr.Get("/ws", s.handleServerWS)

		sr.Group(func(sr chi.Router) {
			sr.Use(s.Sessions.RequireAuth)
			sr.Post("/power", s.handleServerPower)
			sr.Post("/console-token", s.handleConsoleToken)

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

			// server detail & sub-resources (milestone 4)
			sr.Get("/", s.handleServerUserGet)
			sr.Get("/activity", s.handleServerActivity)
			sr.Get("/allocations", s.handleServerAllocationsList)
			sr.Post("/allocations/primary", s.handleServerSetPrimaryAllocation)
		})
	})

	// user server list (GET /api/servers)
	r.Get("/api/servers", wrapH(s.Sessions.RequireAuth(http.HandlerFunc(s.handleUserServersList))))
}

// wrapH: HandlerFunc → http.Handler (helper komposisi middleware chi).
func wrapH(h http.Handler) http.HandlerFunc {
	return h.ServeHTTP
}

// staticHandler serve web/dist via go:embed dengan fallback index.html untuk client-side routing.
func staticHandler() http.HandlerFunc {
	subFS, err := web.DistFS()
	if err != nil {
		panic("dist fs embed: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(subFS))

	indexBytes, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		panic("read embedded index.html: " + err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Unknown /api/* routes → JSON 404, never the SPA fallback.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Cek apakah file ada di subFS
		f, err := subFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback SPA index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	}
}
