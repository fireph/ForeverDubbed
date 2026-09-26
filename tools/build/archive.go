package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func writeZIP(destination string, files map[string]string) (err error) {
	output, err := os.CreateTemp(filepath.Dir(destination), ".foreverdubbed-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	defer output.Close()
	archive := zip.NewWriter(output)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := addFile(archive, name, files[name]); err != nil {
			archive.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), destination)
}

func addFile(archive *zip.Writer, name, source string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name, header.Method = name, zip.Deflate
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, input)
	return err
}
