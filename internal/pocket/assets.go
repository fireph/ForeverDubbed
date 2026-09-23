package pocket

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed assets.json
var assetsJSON []byte

// Asset pins exported model/preset bytes, preventing silent model/voice mismatch.
type Asset struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func Assets() []Asset {
	var assets []Asset
	if err := json.Unmarshal(assetsJSON, &assets); err != nil {
		panic(err)
	}
	return assets
}
func VerifyAsset(dir string, asset Asset) error {
	f, err := os.Open(filepath.Join(dir, filepath.FromSlash(asset.Path)))
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
		return fmt.Errorf("checksum mismatch: %s (run go run ./tools/models)", asset.Path)
	}
	return nil
}
func VerifyModels(dir string) error {
	for _, asset := range Assets() {
		if filepath.Dir(asset.Path) == "models" {
			asset.Path = filepath.Base(asset.Path)
			if err := VerifyAsset(dir, asset); err != nil {
				return err
			}
		}
	}
	return nil
}
