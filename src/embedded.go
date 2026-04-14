package main

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-assets"
)

// Load templates from the embedded assets file
func LoadTemplate(t *template.Template, ext string) error {
	if debug {
		return loadTemplateFromDisk(t, ext)
	}
	for name, file := range Assets.Files {
		if file.IsDir() || !strings.HasSuffix(name, ext) {
			continue
		}
		h, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		t, err = t.New(name).Parse(string(h))
		if err != nil {
			return err
		}
	}
	return nil
}

func loadTemplateFromDisk(t *template.Template, ext string) error {
	root := filepath.Join("..", "htm")
	if ext != ".htm" {
		root = filepath.Join("..")
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ext) {
			return nil
		}

		h, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		name := "/" + strings.TrimPrefix(filepath.ToSlash(path), "../")
		_, err = t.New(name).Parse(string(h))
		return err
	})
}

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
