package dockerrt

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonDecode helper (biar file lain pendek).
func jsonDecode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// _ pakai fmt biar import tidak dibuang saat file berubah.
var _ = fmt.Sprintf
