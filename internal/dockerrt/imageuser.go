// imageuser.go: baca field User dari image config (SPEC §7.4) — dipakai
// supervisor menentukan UID/GID operasi file volume. Bila image tidak
// menyatakan user numeric → fallback 1000:1000 + log (di filecore.OwnerFor).
package dockerrt

import (
	"context"
	"fmt"
)

// ImageUser: field User dari config image (mis. "1000:1000", "nobody", "").
// Return kosong saat inspect gagal (image belum di-pull) — caller fallback.
func (d *dockerRT) ImageUser(ctx context.Context, image string) (string, error) {
	inspect, _, err := d.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return "", fmt.Errorf("inspect image %s: %w", image, err)
	}
	return inspect.Config.User, nil
}
