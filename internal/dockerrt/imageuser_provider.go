// ImageUserProvider: runtime yang bisa membaca field User dari image config
// (SPEC §7.4). Opsional — Docker asli implement; fake untuk test set manual.
package dockerrt

import "context"

// ImageUserProvider opsional: baca User dari image config.
type ImageUserProvider interface {
	ImageUser(ctx context.Context, image string) (string, error)
}
