// Download model exports and matching presets; never needs Python at runtime.
package main

import (
	"context"
	"flag"
	"fmt"
	"foreverdubbed/internal/pocket"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("out", ".runtime/native", "native model/preset directory")
	flag.Parse()
	for _, asset := range pocket.Assets() {
		if pocket.VerifyAsset(*dir, asset) == nil {
			fmt.Println("Verified", asset.Path)
			continue
		}
		if err := download(*dir, asset); err != nil {
			return err
		}
	}
	fmt.Println("Pinned April English models and presets ready; no Python runtime required.")
	return nil
}
func download(dir string, asset pocket.Asset) error {
	dest := filepath.Join(dir, filepath.FromSlash(asset.Path))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	fmt.Println("Downloading", asset.Path)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("%s: %s", asset.Path, response.Status)
	}
	temp, err := os.CreateTemp(filepath.Dir(dest), "download-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if _, err = io.Copy(temp, io.LimitReader(response.Body, 512<<20)); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	check := asset
	check.Path = filepath.Base(temp.Name())
	if err = pocket.VerifyAsset(filepath.Dir(temp.Name()), check); err != nil {
		return err
	}
	return os.Rename(temp.Name(), dest)
}
