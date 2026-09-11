package main

import (
	"archive/zip"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/image/draw"
)

type ZipFile struct {
	zipFile *zip.ReadCloser
}

func (zf *ZipFile) Open() {
	if zf.zipFile != nil {
		return
	}
	zr, err := zip.OpenReader(filepath.Join(ConfigFolder, "smcontent.zip"))
	zf.zipFile = zr

	if err != nil {
		log.Print("Could not open smcontent.zip", err)
	}
}

func (zf *ZipFile) Close() {
	if zf.zipFile == nil {
		return
	}
	err := zf.zipFile.Close()
	zf.zipFile = nil
	if err != nil {
		log.Print("Could not close smcontent.zip", err)
	}
}

// Find a file in smcontent.zip
func (zf *ZipFile) FindZipFile(filePath string) *zip.File {
	zf.Open()
	if zf.zipFile == nil {
		return nil
	}

	for _, z := range zf.zipFile.File {
		if z.Name == filePath {
			return z
		}
	}
	return nil
}

// Find files in smcontent.zip, ignoring image files
func (zf *ZipFile) FindZipFiles(filePath string) []*zip.File {
	var zi []*zip.File
	zf.Open()
	if zf.zipFile == nil {
		return zi
	}

	for _, z := range zf.zipFile.File {
		if strings.HasPrefix(z.Name, filePath) && !strings.HasSuffix(z.Name, ".jpg") && !strings.HasSuffix(z.Name, ".jpeg") && !strings.HasSuffix(z.Name, ".png") {
			zi = append(zi, z)
		}
	}

	return zi
}

// Extract a zip item to a given folder
func (zf *ZipFile) ExtractFile(f *zip.File, filePath string) {
	if f == nil {
		return
	}
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			log.Print("Error creating directory: ", err)
		}
		return
	}

	mkdir := filepath.Join(filePath, filepath.Dir(f.Name))
	if err := os.MkdirAll(mkdir, os.ModePerm); err != nil {
		log.Print("Error creating directory: ", err)
	}

	destinationFile, err := os.OpenFile(filepath.Join(filePath, f.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		log.Print("Error creating file: ", err)
	}
	defer destinationFile.Close()

	zippedFile, err := f.Open()
	if err != nil {
		log.Print("Error opening file: ", err)
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		log.Print("Error extracting file: ", err)
	}
}

func (zf *ZipFile) ExtractFileToSubfolder(f *zip.File, filePath string) {
	if f == nil {
		return
	}
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			log.Print("Error creating directory: ", err)
		}
		return
	}

	if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
		log.Print("Error creating directory: ", err)
	}

	destinationFile, err := os.OpenFile(filepath.Join(filePath, filepath.Base(f.Name)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		log.Print("Error creating file: ", err)
	}
	defer destinationFile.Close()

	zippedFile, err := f.Open()
	if err != nil {
		log.Print("Error opening file: ", err)
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		log.Print("Error extracting file: ", err)
	}
}

func (zf *ZipFile) ExtractFiles(zi []*zip.File, filePath string) {
	for _, z := range zi {
		zf.ExtractFile(z, filePath)
	}
}

// Rebuild from the current inventory, retaining unchanged compressed entries.
// Close the complete temporary archive before atomically replacing the old one.
func (zf *ZipFile) UpdateZipfile(filesToZip map[string]string) error {
	filename := filepath.Join(ConfigFolder, "smcontent.zip")
	previous, err := zip.OpenReader(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	old := map[string]*zip.File{}
	if previous != nil {
		defer previous.Close()
		for _, entry := range previous.File {
			old[entry.Name] = entry
		}
	}
	temp, err := os.CreateTemp(ConfigFolder, "smcontent-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	writer := zip.NewWriter(temp)
	defer writer.Close()
	keys := make([]string, 0, len(filesToZip))
	for key := range filesToZip {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, path := range keys {
		if err = writeContentEntry(writer, old[filesToZip[path]], path, filesToZip[path]); err != nil {
			return err
		}
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if previous != nil {
		previous.Close()
	}
	zf.Close()
	return os.Rename(temp.Name(), filename)
}

func writeContentEntry(writer *zip.Writer, old *zip.File, path, destination string) error {
	source, err := os.Open(path)
	// The parser includes both Windows and Linux executables. Only the installed
	// platform is required; launch readiness validates its executable explicitly.
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("Content archive: absent file %s", path)
		return nil
	}
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if old != nil && old.Modified.Unix() == info.ModTime().Unix() {
		return writer.Copy(old)
	}
	header := &zip.FileHeader{Name: destination, Method: zip.Deflate, Modified: info.ModTime()}
	output, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".jpg" || extension == ".jpeg" || extension == ".png" {
		img, _, decodeErr := image.Decode(source)
		if decodeErr == nil {
			width := min(640, img.Bounds().Dx())
			height := max(1, int(float64(img.Bounds().Dy())/float64(img.Bounds().Dx())*float64(width)))
			resized := image.NewRGBA(image.Rect(0, 0, width, height))
			draw.BiLinear.Scale(resized, resized.Rect, img, img.Bounds(), draw.Over, nil)
			if extension == ".png" {
				return png.Encode(output, resized)
			}
			return jpeg.Encode(output, resized, &jpeg.Options{Quality: jpeg.DefaultQuality})
		}
		if _, err = source.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}
	_, err = io.Copy(output, source)
	return err
}
