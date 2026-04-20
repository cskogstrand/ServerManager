package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const maxContentUploadSize int64 = 2 << 30

type contentCounts struct {
	Tracks   int
	Cars     int
	Weathers int
}

type contentUploadError struct {
	Status  int
	Message string
}

func (e contentUploadError) Error() string {
	return e.Message
}

type contentArchiveImportResult struct {
	AssetKeys    []string
	FilesWritten int
}

type detectedArchiveAsset struct {
	Key      string
	RootPath string
}

func currentContentCounts() (contentCounts, error) {
	tracks, err := Dba.selectCacheTracks()
	if err != nil {
		return contentCounts{}, err
	}

	cars, err := Dba.selectCacheCars()
	if err != nil {
		return contentCounts{}, err
	}

	weathers, err := Dba.selectCacheWeathers()
	if err != nil {
		return contentCounts{}, err
	}

	return contentCounts{
		Tracks:   len(tracks),
		Cars:     len(cars),
		Weathers: len(weathers),
	}, nil
}

func refreshContentCounts() (contentCounts, error) {
	if _, err := Dba.basepath(); err != nil {
		return contentCounts{}, err
	}
	parseContent(Dba)
	return currentContentCounts()
}

func parseBoolFormValue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func contentKindFolder(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "car":
		return "cars", nil
	case "track":
		return "tracks", nil
	default:
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Select either car or track before uploading.",
		}
	}
}

func importContentArchive(zipPath string, basepath string, kind string, overwrite bool) (contentArchiveImportResult, error) {
	contentFolder, err := contentKindFolder(kind)
	if err != nil {
		return contentArchiveImportResult{}, err
	}

	destinationRoot := filepath.Join(basepath, "content", contentFolder)
	if err := os.MkdirAll(destinationRoot, os.ModePerm); err != nil {
		return contentArchiveImportResult{}, fmt.Errorf("could not prepare destination folder: %w", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return contentArchiveImportResult{}, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The selected file is not a valid zip archive.",
		}
	}
	defer reader.Close()

	assets, err := detectArchiveAssets(reader.File, kind)
	if err != nil {
		return contentArchiveImportResult{}, err
	}

	if !overwrite {
		conflicts := make([]string, 0)
		for _, asset := range assets {
			if _, err := os.Stat(filepath.Join(destinationRoot, asset.Key)); err == nil {
				conflicts = append(conflicts, asset.Key)
			}
		}
		if len(conflicts) > 0 {
			sort.Strings(conflicts)
			return contentArchiveImportResult{}, contentUploadError{
				Status:  http.StatusConflict,
				Message: "Archive already exists on disk: " + strings.Join(conflicts, ", ") + ". Enable overwrite to replace it.",
			}
		}
	}

	stagingRoot, err := os.MkdirTemp(destinationRoot, ".sm-upload-*")
	if err != nil {
		return contentArchiveImportResult{}, fmt.Errorf("could not create staging folder: %w", err)
	}
	defer os.RemoveAll(stagingRoot)

	filesWritten := 0
	for _, file := range reader.File {
		trimmed, skip, err := trimmedArchivePath(kind, file.Name)
		if err != nil {
			return contentArchiveImportResult{}, err
		}
		if skip {
			continue
		}

		mapped, ok := mapArchiveFileToAsset(trimmed, assets)
		if !ok {
			continue
		}

		targetPath, err := safeJoinUnderRoot(stagingRoot, filepath.FromSlash(mapped))
		if err != nil {
			return contentArchiveImportResult{}, err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, os.ModePerm); err != nil {
				return contentArchiveImportResult{}, fmt.Errorf("could not create folder: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not prepare extracted file path: %w", err)
		}

		src, err := file.Open()
		if err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not read archive entry %q: %w", file.Name, err)
		}

		mode := file.Mode()
		if mode == 0 {
			mode = 0o644
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			src.Close()
			return contentArchiveImportResult{}, fmt.Errorf("could not create extracted file: %w", err)
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return contentArchiveImportResult{}, fmt.Errorf("could not extract archive entry %q: %w", file.Name, err)
		}
		if err := dst.Close(); err != nil {
			src.Close()
			return contentArchiveImportResult{}, fmt.Errorf("could not finalize extracted file: %w", err)
		}
		if err := src.Close(); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not close archive entry %q: %w", file.Name, err)
		}

		filesWritten++
	}

	for _, asset := range assets {
		sourcePath := filepath.Join(stagingRoot, asset.Key)
		if _, err := os.Stat(sourcePath); errors.Is(err, os.ErrNotExist) {
			continue
		}

		targetPath := filepath.Join(destinationRoot, asset.Key)
		if overwrite {
			if err := os.RemoveAll(targetPath); err != nil {
				return contentArchiveImportResult{}, fmt.Errorf("could not replace existing content: %w", err)
			}
		}

		if err := os.Rename(sourcePath, targetPath); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not install %q: %w", asset.Key, err)
		}
	}

	keys := make([]string, 0, len(assets))
	for _, asset := range assets {
		keys = append(keys, asset.Key)
	}
	sort.Strings(keys)

	return contentArchiveImportResult{
		AssetKeys:   keys,
		FilesWritten: filesWritten,
	}, nil
}

func detectArchiveAssets(files []*zip.File, kind string) ([]detectedArchiveAsset, error) {
	assets := make([]detectedArchiveAsset, 0)

	for _, file := range files {
		trimmed, skip, err := trimmedArchivePath(kind, file.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}

		rootPath, assetKey, ok := detectArchiveAssetRoot(trimmed, kind)
		if !ok {
			continue
		}

		index := indexAssetByKey(assets, assetKey)
		if index >= 0 {
			if assets[index].RootPath != rootPath {
				return nil, contentUploadError{
					Status:  http.StatusBadRequest,
					Message: "Archive contains multiple conflicting folders for " + assetKey + ".",
				}
			}
			continue
		}

		assets = append(assets, detectedArchiveAsset{
			Key:      assetKey,
			RootPath: rootPath,
		})
	}

	if len(assets) == 0 {
		expected := "Expected a zip that contains content/" + contentFolderForMessage(kind) + "/... or a direct mod folder with the standard Assetto Corsa structure."
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "No valid " + kind + " content was found in the archive. " + expected,
		}
	}

	return assets, nil
}

func trimmedArchivePath(kind string, archivePath string) (string, bool, error) {
	cleaned, skip, err := normalizeArchivePath(archivePath)
	if err != nil || skip {
		return "", skip, err
	}

	segments := strings.Split(cleaned, "/")
	folder := contentFolderForMessage(kind)
	if idx := indexSegmentSequence(segments, []string{"content", folder}); idx >= 0 {
		trimmed := strings.Join(segments[idx+2:], "/")
		if trimmed == "" {
			return "", true, nil
		}
		return trimmed, false, nil
	}
	if idx := indexSegmentSequence(segments, []string{folder}); idx >= 0 {
		trimmed := strings.Join(segments[idx+1:], "/")
		if trimmed == "" {
			return "", true, nil
		}
		return trimmed, false, nil
	}

	return cleaned, false, nil
}

func normalizeArchivePath(archivePath string) (string, bool, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(archivePath), "\\", "/")
	if normalized == "" {
		return "", true, nil
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" {
		return "", true, nil
	}
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Archive contains an invalid file path and cannot be extracted safely.",
		}
	}
	if strings.HasPrefix(cleaned, "__MACOSX/") || path.Base(cleaned) == ".DS_Store" || strings.HasPrefix(path.Base(cleaned), "._") {
		return "", true, nil
	}

	return cleaned, false, nil
}

func detectArchiveAssetRoot(trimmed string, kind string) (string, string, bool) {
	segments := strings.Split(trimmed, "/")
	filename := segments[len(segments)-1]

	for i := 1; i < len(segments); i++ {
		if segments[i] != "ui" {
			continue
		}
		if kind == "car" && i == len(segments)-2 && (filename == "ui_car.json" || filename == "dlc_ui_car.json") {
			return strings.Join(segments[:i], "/"), segments[i-1], true
		}
		if kind == "track" && (filename == "ui_track.json" || filename == "dlc_ui_track.json") {
			return strings.Join(segments[:i], "/"), segments[i-1], true
		}
	}

	return "", "", false
}

func mapArchiveFileToAsset(trimmed string, assets []detectedArchiveAsset) (string, bool) {
	for _, asset := range assets {
		if trimmed == asset.RootPath || strings.HasPrefix(trimmed, asset.RootPath+"/") {
			suffix := strings.TrimPrefix(trimmed, asset.RootPath)
			suffix = strings.TrimPrefix(suffix, "/")
			if suffix == "" {
				return asset.Key, true
			}
			return asset.Key + "/" + suffix, true
		}
	}

	return "", false
}

func safeJoinUnderRoot(root string, rel string) (string, error) {
	targetPath := filepath.Clean(filepath.Join(root, rel))
	rootPath := filepath.Clean(root)

	if targetPath != rootPath && !strings.HasPrefix(targetPath, rootPath+string(os.PathSeparator)) {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Archive contains an invalid file path and cannot be extracted safely.",
		}
	}

	return targetPath, nil
}

func indexSegmentSequence(segments []string, needle []string) int {
	if len(needle) == 0 || len(segments) < len(needle) {
		return -1
	}

	for i := 0; i <= len(segments)-len(needle); i++ {
		if segmentSequenceEqual(segments[i:i+len(needle)], needle) {
			return i
		}
	}

	return -1
}

func indexAssetByKey(assets []detectedArchiveAsset, key string) int {
	for i, asset := range assets {
		if asset.Key == key {
			return i
		}
	}

	return -1
}

func segmentSequenceEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func contentFolderForMessage(kind string) string {
	if kind == "track" {
		return "tracks"
	}
	return "cars"
}
