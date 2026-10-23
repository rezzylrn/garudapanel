// owner_unix.go: pembacaan UID/GID user di dalam container image (SPEC §7.4).
// Sumber: image config (field User dari inspect image/container).
// Format User: "uid", "uid:gid", "user", "user:group" (nama → resolve di
// daemon asli via getent di dalam container; di sini numeric saja).
// Fallback 1000:1000 + log (file yang ditulis panel tidak boleh root-owned).
package filecore

import (
	"log"
	"strconv"
	"strings"
)

// DefaultUIDGID fallback bila image tidak menyatakan user numeric.
const (
	DefaultUID = 1000
	DefaultGID = 1000
)

// ParseImageUser parse field User image/container config → (uid, gid, ok).
// ok=false bila tidak numeric (mis. "nobody" atau kosong) — caller pakai
// fallback dan mencatat log.
func ParseImageUser(user string) (uid, gid int, ok bool) {
	if user == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(user, ":", 2)
	uidStr := parts[0]
	gidStr := uidStr
	if len(parts) == 2 {
		gidStr = parts[1]
	}
	u, err := strconv.Atoi(uidStr)
	if err != nil || u < 0 {
		return 0, 0, false
	}
	g := u
	if g2, err := strconv.Atoi(gidStr); err == nil && g2 >= 0 {
		g = g2
	}
	return u, g, true
}

// OwnerFor: tentukan UID/GID operasi I/O untuk server dengan imageUser
// (field User dari image config, bisa kosong). Fallback 1000:1000 + log.
func OwnerFor(serverUUID, imageUser string) (uid, gid int) {
	if uid, gid, ok := ParseImageUser(imageUser); ok {
		return uid, gid
	}
	if imageUser != "" {
		log.Printf("filecore: server %s: image user %q non-numeric; fallback %d:%d",
			serverUUID, imageUser, DefaultUID, DefaultGID)
	}
	return DefaultUID, DefaultGID
}
