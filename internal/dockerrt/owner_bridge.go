// Jembatan dockerrt → filecore: FileOwner di ContainerSpec. Package
// terpisah supaya dockerrt tidak import filecore (arah dependensi tetap
// daemon → dockerrt, filecore mandiri).
package dockerrt

import "garudapanel/internal/filecore"

// filecoreOwnerFor delegasi ke filecore.OwnerFor (fallback 1000:1000 + log).
func filecoreOwnerFor(uuid, imageUser string) (int, int) {
	return filecore.OwnerFor(uuid, imageUser)
}
