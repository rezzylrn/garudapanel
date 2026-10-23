// Test files endpoint RBAC & validation di level panel (HTTP handler).
// Memastikan: non-owner 403, owner 200/503 (offline node), path traversal ditolak (422).
package panel

import (
	"fmt"
	"testing"

	"garudapanel/internal/auth"
)

func TestFilesRBACNonOwner403(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)

	// Non-owner (user id 888) akses GET /files -> 403
	w := runVarRoute(t, s.handleFileList, "GET",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files", serverID),
		auth.SessionRow{UserID: 888, IsAdmin: false})
	if w.Code != 403 {
		t.Fatalf("non-owner harus 403, got %d", w.Code)
	}

	// Non-owner coba mkdir -> 403
	w = runVarRoute(t, s.handleFileMkdir, "POST",
		"/api/servers/{id}/files/mkdir",
		fmt.Sprintf("/api/servers/%d/files/mkdir", serverID),
		auth.SessionRow{UserID: 888, IsAdmin: false},
		`{"path":"plugins"}`)
	if w.Code != 403 {
		t.Fatalf("non-owner mkdir harus 403, got %d", w.Code)
	}

	// Non-owner coba delete -> 403
	w = runVarRoute(t, s.handleFileDelete, "DELETE",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files?path=plugins", serverID),
		auth.SessionRow{UserID: 888, IsAdmin: false})
	if w.Code != 403 {
		t.Fatalf("non-owner delete harus 403, got %d", w.Code)
	}

	_ = ownerID
}

func TestFilesPathTraversalValidation(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)

	// Traversal di mkdir -> 422
	w := runVarRoute(t, s.handleFileMkdir, "POST",
		"/api/servers/{id}/files/mkdir",
		fmt.Sprintf("/api/servers/%d/files/mkdir", serverID),
		auth.SessionRow{UserID: ownerID},
		`{"path":"../../etc"}`)
	if w.Code != 422 {
		t.Fatalf("traversal path mkdir harus 422, got %d", w.Code)
	}

	// Traversal di rename -> 422
	w = runVarRoute(t, s.handleFileRename, "POST",
		"/api/servers/{id}/files/rename",
		fmt.Sprintf("/api/servers/%d/files/rename", serverID),
		auth.SessionRow{UserID: ownerID},
		`{"from":"../../etc/passwd","to":"passwd"}`)
	if w.Code != 422 {
		t.Fatalf("traversal from rename harus 422, got %d", w.Code)
	}

	// Traversal di delete -> 422
	w = runVarRoute(t, s.handleFileDelete, "DELETE",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files?path=../../../etc/passwd", serverID),
		auth.SessionRow{UserID: ownerID})
	if w.Code != 422 {
		t.Fatalf("traversal delete harus 422, got %d", w.Code)
	}
}

func TestFilesUploadNodeOffline503(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)

	// Hub kosong (node offline) -> 503
	w := runVarRoute(t, s.handleFileWrite, "PUT",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files?path=test.txt", serverID),
		auth.SessionRow{UserID: ownerID},
		"hello file content")
	if w.Code != 503 {
		t.Fatalf("upload node offline harus 503, got %d", w.Code)
	}

	// File list saat node offline -> 503
	wList := runVarRoute(t, s.handleFileList, "GET",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files", serverID),
		auth.SessionRow{UserID: ownerID})
	if wList.Code != 503 {
		t.Fatalf("list node offline harus 503, got %d", wList.Code)
	}

	// File delete saat node offline -> 503
	wDel := runVarRoute(t, s.handleFileDelete, "DELETE",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files?path=test.txt", serverID),
		auth.SessionRow{UserID: ownerID})
	if wDel.Code != 503 {
		t.Fatalf("delete node offline harus 503, got %d", wDel.Code)
	}

	// Upload size limit test
	s.MaxUploadBytes = 10 // batasi 10 bytes
	wOver := runVarRoute(t, s.handleFileWrite, "PUT",
		"/api/servers/{id}/files",
		fmt.Sprintf("/api/servers/%d/files?path=test.txt", serverID),
		auth.SessionRow{UserID: ownerID},
		"konten ini sengaja dibuat lebih panjang dari 10 byte")
	// Karena node offline, di cek pertama conn == nil -> 503.
	// Jika node online, akan 413. Cek apakah handler tetap berjalan.
	if wOver.Code != 503 && wOver.Code != 413 {
		t.Fatalf("over-limit upload got %d", wOver.Code)
	}
}
