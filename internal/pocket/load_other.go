//go:build !pocket_native || !cgo || (!windows && !linux && !darwin)

package pocket

import "fmt"

func openLibrary() (library, error) {
	return nil, fmt.Errorf("PocketTTS requires a build with CGO_ENABLED=1 and -tags pocket_native; prepare dependencies with go run ./tools/native")
}
