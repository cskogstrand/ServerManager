package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-assets"
)

// Find a file in the embedded assets file
func FindFile(filePath string) *assets.File {
	for _, file := range Assets.Files {
		if file.Path == filePath {
			return file
		}
	}
	return nil
}

func OpenAsset(filePath string) (io.ReadCloser, error) {
	if debug {
		diskPath := filepath.Join("..", strings.TrimPrefix(filepath.FromSlash(filePath), "/"))
		return os.Open(diskPath)
	}

	file := FindFile(filePath)
	if file == nil {
		return nil, os.ErrNotExist
	}

	return file, nil
}
