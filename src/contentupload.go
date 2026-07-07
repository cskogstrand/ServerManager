package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxContentUploadSize int64 = 10 << 30
const contentDownloadTimeout = 20 * time.Minute

var supportedArchiveExtensions = []string{
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tgz",
	".tbz2",
	".txz",
	".zip",
	".rar",
	".7z",
	".tar",
	".gz",
	".bz2",
	".xz",
}

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
	Kind         string
	AssetKeys    []string
	FilesWritten int
}

func touchInstalledAsset(path string) error {
	now := time.Now()
	return os.Chtimes(path, now, now)
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
			Message: "Only car and track content uploads are supported.",
		}
	}
}

func archiveExtensionForPath(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, ext := range supportedArchiveExtensions {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return ""
}

func archiveExtensionForContentType(mediaType string) string {
	switch mediaType {
	case "application/zip", "application/x-zip-compressed":
		return ".zip"
	case "application/vnd.rar", "application/x-rar-compressed":
		return ".rar"
	case "application/x-7z-compressed":
		return ".7z"
	case "application/x-tar":
		return ".tar"
	case "application/gzip", "application/x-gzip":
		return ".gz"
	case "application/x-bzip2":
		return ".bz2"
	case "application/x-xz":
		return ".xz"
	default:
		return ""
	}
}

func archiveRequires7z(ext string) bool {
	return ext != "" && ext != ".zip"
}

func ensureSupportedArchiveName(name string) (string, error) {
	ext := archiveExtensionForPath(name)
	if ext == "" {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Unsupported archive type. Supported formats: zip, rar, 7z, tar, tar.gz, tgz, tar.bz2, tbz2, tar.xz, txz, gz, bz2, xz.",
		}
	}
	return ext, nil
}

func newTempArchiveFile(sourceName string) (string, error) {
	ext := archiveExtensionForPath(sourceName)
	pattern := "sm-upload-*"
	if ext != "" {
		pattern += ext
	}

	tempFile, err := os.CreateTemp(TempFolder, pattern)
	if err != nil {
		return "", err
	}

	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return "", err
	}

	return tempPath, nil
}

func archiveNameFromContentDisposition(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}

	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}

	if filenameStar := strings.TrimSpace(params["filename*"]); filenameStar != "" {
		parts := strings.SplitN(filenameStar, "''", 2)
		if len(parts) == 2 {
			if decoded, err := url.QueryUnescape(parts[1]); err == nil {
				return filepath.Base(decoded)
			}
		}
		return filepath.Base(filenameStar)
	}

	if filename := strings.TrimSpace(params["filename"]); filename != "" {
		return filepath.Base(filename)
	}

	return ""
}

func detectArchiveNameFromFile(path string, fallback string) string {
	if ext := archiveExtensionForPath(fallback); ext != "" {
		base := strings.TrimSuffix(filepath.Base(fallback), ext)
		if base == "" || base == "." || base == "/" {
			base = "download"
		}
		return base + ext
	}

	file, err := os.Open(path)
	if err != nil {
		return "download.zip"
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "download.zip"
	}
	buffer = buffer[:n]

	switch {
	case len(buffer) >= 4 && string(buffer[:2]) == "PK":
		return "download.zip"
	case len(buffer) >= 8 && string(buffer[:8]) == "Rar!\x1a\x07\x00":
		return "download.rar"
	case len(buffer) >= 8 && string(buffer[:8]) == "Rar!\x1a\x07\x01":
		return "download.rar"
	case len(buffer) >= 6 && buffer[0] == 0x37 && buffer[1] == 0x7A && buffer[2] == 0xBC && buffer[3] == 0xAF && buffer[4] == 0x27 && buffer[5] == 0x1C:
		return "download.7z"
	case len(buffer) >= 2 && buffer[0] == 0x1F && buffer[1] == 0x8B:
		return "download.gz"
	case len(buffer) >= 3 && string(buffer[:3]) == "BZh":
		return "download.bz2"
	case len(buffer) >= 6 && buffer[0] == 0xFD && buffer[1] == 0x37 && buffer[2] == 0x7A && buffer[3] == 0x58 && buffer[4] == 0x5A && buffer[5] == 0x00:
		return "download.xz"
	case len(buffer) >= 265 && string(buffer[257:262]) == "ustar":
		return "download.tar"
	default:
		return "download.zip"
	}
}

func buildContentJobDownloadMessage(downloaded int64, total int64) string {
	if total > 0 {
		return fmt.Sprintf("Downloading archive (%s / %s)...", formatByteCount(downloaded), formatByteCount(total))
	}
	return fmt.Sprintf("Downloading archive (%s)...", formatByteCount(downloaded))
}

func formatByteCount(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}

	if unit == 0 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.3f %s", value, units[unit])
}

func find7zBinary() string {
	for _, name := range []string{"7z", "7zz", "7za"} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return path
		}
	}
	return ""
}

func ensureArchiveExtractor(ext string) (string, error) {
	if !archiveRequires7z(ext) {
		return "", nil
	}

	binary := find7zBinary()
	if binary == "" {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "This archive format uses the 7-Zip binary available in the current runtime. No embedded 7-Zip library is bundled. If you are running in Docker, rebuild/restart the container so the packaged `7z` binary is present; otherwise install `7z`, `7zz`, or `7za` in this environment.",
		}
	}

	return binary, nil
}

func validateArchiveURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The archive URL is not valid.",
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Only http and https archive URLs are supported.",
		}
	}
	if parsed.Host == "" {
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The archive URL must include a host.",
		}
	}

	return parsed, nil
}

func isArchiveContentType(mediaType string) bool {
	switch mediaType {
	case "application/zip",
		"application/x-zip-compressed",
		"application/vnd.rar",
		"application/x-rar-compressed",
		"application/x-7z-compressed",
		"application/x-tar",
		"application/gzip",
		"application/x-gzip",
		"application/x-bzip2",
		"application/x-xz",
		"application/octet-stream":
		return true
	}

	return strings.Contains(mediaType, "zip") ||
		strings.Contains(mediaType, "rar") ||
		strings.Contains(mediaType, "7z") ||
		strings.Contains(mediaType, "tar") ||
		strings.Contains(mediaType, "gzip") ||
		strings.Contains(mediaType, "bzip") ||
		strings.Contains(mediaType, "xz")
}

func downloadContentArchive(rawURL string, destinationPath string, progress func(downloaded int64, total int64)) (string, error) {
	archiveURL, err := validateArchiveURL(rawURL)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), contentDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("could not create download request: %w", err)
	}
	req.Header.Set("User-Agent", "ServerManager/ContentImport")

	client := &http.Client{
		Timeout: contentDownloadTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", contentUploadError{
			Status:  http.StatusBadGateway,
			Message: "Could not download the archive URL.",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", contentUploadError{
			Status:  http.StatusBadGateway,
			Message: fmt.Sprintf("Archive download failed with HTTP %d.", resp.StatusCode),
		}
	}
	if resp.ContentLength > maxContentUploadSize {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The remote archive is larger than 10 GB.",
		}
	}

	resolvedName := archiveNameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType != "" {
		mediaType, _, parseErr := mime.ParseMediaType(contentType)
		if parseErr == nil && !isArchiveContentType(mediaType) {
			return "", contentUploadError{
				Status:  http.StatusBadRequest,
				Message: "The remote URL did not return a supported archive.",
			}
		}
		if resolvedName == "" {
			if ext := archiveExtensionForContentType(mediaType); ext != "" {
				resolvedName = "download" + ext
			}
		}
	}

	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not prepare downloaded archive: %w", err)
	}
	defer destinationFile.Close()

	if progress != nil {
		progress(0, resp.ContentLength)
	}

	written := int64(0)
	buffer := make([]byte, 1024*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			written += int64(n)
			if written > maxContentUploadSize {
				return "", contentUploadError{
					Status:  http.StatusBadRequest,
					Message: "The remote archive is larger than 10 GB.",
				}
			}

			if _, err := destinationFile.Write(buffer[:n]); err != nil {
				return "", contentUploadError{
					Status:  http.StatusBadGateway,
					Message: "The archive download was interrupted.",
				}
			}
			if progress != nil {
				progress(written, resp.ContentLength)
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", contentUploadError{
				Status:  http.StatusBadGateway,
				Message: "The archive download was interrupted.",
			}
		}
	}
	if written > maxContentUploadSize {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The remote archive is larger than 10 GB.",
		}
	}

	return detectArchiveNameFromFile(destinationPath, resolvedName), nil
}

func importContentArchive(archivePath string, archiveName string, basepath string, kind string, overwrite bool) (contentArchiveImportResult, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "auto"
	}
	ext, err := ensureSupportedArchiveName(archiveName)
	if err != nil {
		return contentArchiveImportResult{}, err
	}
	if kind == "auto" {
		detected, err := detectArchiveContentKind(archivePath, ext)
		if err != nil {
			return contentArchiveImportResult{}, err
		}
		kind = detected
	}
	contentFolder, err := contentKindFolder(kind)
	if err != nil {
		return contentArchiveImportResult{}, err
	}

	destinationRoot := filepath.Join(basepath, "content", contentFolder)
	if err := os.MkdirAll(destinationRoot, os.ModePerm); err != nil {
		return contentArchiveImportResult{}, fmt.Errorf("could not prepare destination folder: %w", err)
	}

	if ext == ".zip" {
		result, err := importZipArchive(archivePath, destinationRoot, kind, overwrite)
		result.Kind = kind
		return result, err
	}

	sevenZipBinary, err := ensureArchiveExtractor(ext)
	if err != nil {
		return contentArchiveImportResult{}, err
	}

	result, err := importArchiveVia7z(archivePath, destinationRoot, kind, overwrite, sevenZipBinary)
	result.Kind = kind
	return result, err
}

func detectArchiveContentKind(archivePath string, ext string) (string, error) {
	if ext == ".zip" {
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return "", contentUploadError{
				Status:  http.StatusBadRequest,
				Message: "The selected archive could not be opened.",
			}
		}
		defer reader.Close()

		paths := make([]string, 0, len(reader.File))
		for _, file := range reader.File {
			paths = append(paths, file.Name)
		}
		return detectArchiveContentKindFromPaths(paths)
	}

	sevenZipBinary, err := ensureArchiveExtractor(ext)
	if err != nil {
		return "", err
	}
	paths, err := listArchivePathsVia7z(archivePath, sevenZipBinary)
	if err != nil {
		return "", err
	}
	return detectArchiveContentKindFromPaths(paths)
}

func listArchivePathsVia7z(archivePath string, sevenZipBinary string) ([]string, error) {
	cmd := exec.Command(sevenZipBinary, "l", "-slt", archivePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: friendly7zArchiveError(string(output), "7-Zip could not inspect the archive."),
		}
	}

	return parse7zListPaths(string(output)), nil
}

func friendly7zArchiveError(output string, fallback string) string {
	msg := strings.TrimSpace(output)
	lower := strings.ToLower(msg)
	if strings.Contains(msg, "Cannot open the file as archive") {
		return "The archive could not be opened. It may be corrupt, incomplete, password-protected, an unsupported RAR variant, or one part of a multi-part RAR."
	}
	if strings.Contains(lower, "wrong password") || strings.Contains(lower, "encrypted") {
		return "The archive is password-protected or encrypted and cannot be imported."
	}
	if strings.Contains(lower, "unexpected end") || strings.Contains(lower, "headers error") {
		return "The archive appears to be incomplete or corrupt."
	}
	if msg == "" {
		return fallback
	}
	if strings.Contains(msg, "ERROR:") || strings.Contains(msg, "7-Zip") {
		return fallback
	}
	return msg
}

func parse7zListPaths(output string) []string {
	paths := make([]string, 0)
	inFileList := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "----------") {
			inFileList = true
			continue
		}
		if !inFileList {
			continue
		}
		if path, ok := strings.CutPrefix(line, "Path = "); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

func detectArchiveContentKindFromPaths(paths []string) (string, error) {
	found := map[string]bool{}
	for _, archivePath := range paths {
		for _, kind := range []string{"car", "track"} {
			if _, ok, err := detectFilesystemAsset(kind, archivePath); err != nil {
				return "", err
			} else if ok {
				found[kind] = true
			}
		}
	}

	if found["car"] && found["track"] {
		return "", contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "Archive contains both car and track content. Upload mixed content as separate archives so each file can be installed to the right folder.",
		}
	}
	if found["car"] {
		return "car", nil
	}
	if found["track"] {
		return "track", nil
	}
	return "", contentUploadError{
		Status:  http.StatusBadRequest,
		Message: "No valid car or track content was found in the archive. Expected an Assetto Corsa car with ui/ui_car.json or track with ui/ui_track.json.",
	}
}

func importZipArchive(archivePath string, destinationRoot string, kind string, overwrite bool) (contentArchiveImportResult, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return contentArchiveImportResult{}, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "The selected archive could not be opened.",
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
		if err := touchInstalledAsset(targetPath); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not timestamp %q: %w", asset.Key, err)
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
		expected := "Expected an archive that contains content/" + contentFolderForMessage(kind) + "/... or a direct mod folder with the standard Assetto Corsa structure."
		return nil, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "No valid " + kind + " content was found in the archive. " + expected,
		}
	}

	return assets, nil
}

func importArchiveVia7z(archivePath string, destinationRoot string, kind string, overwrite bool, sevenZipBinary string) (contentArchiveImportResult, error) {
	stagingRoot, err := os.MkdirTemp(destinationRoot, ".sm-upload-*")
	if err != nil {
		return contentArchiveImportResult{}, fmt.Errorf("could not create staging folder: %w", err)
	}
	defer os.RemoveAll(stagingRoot)

	extractRoot := filepath.Join(stagingRoot, "extract")
	if err := os.MkdirAll(extractRoot, os.ModePerm); err != nil {
		return contentArchiveImportResult{}, fmt.Errorf("could not prepare extraction folder: %w", err)
	}

	cmd := exec.Command(sevenZipBinary, "x", "-y", "-bso0", "-bsp0", "-bse1", "-o"+extractRoot, archivePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return contentArchiveImportResult{}, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: friendly7zArchiveError(string(output), "7-Zip could not extract the archive."),
		}
	}

	assets, err := detectExtractedAssets(extractRoot, kind)
	if err != nil {
		return contentArchiveImportResult{}, err
	}
	if len(assets) == 0 {
		expected := "Expected an archive that contains content/" + contentFolderForMessage(kind) + "/... or a direct mod folder with the standard Assetto Corsa structure."
		return contentArchiveImportResult{}, contentUploadError{
			Status:  http.StatusBadRequest,
			Message: "No valid " + kind + " content was found in the archive. " + expected,
		}
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

	filesWritten := 0
	keys := make([]string, 0, len(assets))
	for _, asset := range assets {
		sourcePath := filepath.Join(extractRoot, filepath.FromSlash(asset.RootPath))
		targetPath := filepath.Join(destinationRoot, asset.Key)

		count, err := countFilesInTree(sourcePath)
		if err != nil {
			return contentArchiveImportResult{}, err
		}
		filesWritten += count

		if overwrite {
			if err := os.RemoveAll(targetPath); err != nil {
				return contentArchiveImportResult{}, fmt.Errorf("could not replace existing content: %w", err)
			}
		}

		if err := os.Rename(sourcePath, targetPath); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not install %q: %w", asset.Key, err)
		}
		if err := touchInstalledAsset(targetPath); err != nil {
			return contentArchiveImportResult{}, fmt.Errorf("could not timestamp %q: %w", asset.Key, err)
		}
		keys = append(keys, asset.Key)
	}

	sort.Strings(keys)
	return contentArchiveImportResult{
		AssetKeys:    keys,
		FilesWritten: filesWritten,
	}, nil
}

func detectExtractedAssets(root string, kind string) ([]detectedArchiveAsset, error) {
	assets := make([]detectedArchiveAsset, 0)
	err := filepath.Walk(root, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, currentPath)
		if err != nil {
			return err
		}

		asset, ok, err := detectFilesystemAsset(kind, filepath.ToSlash(relPath))
		if err != nil || !ok {
			return err
		}

		index := indexAssetByKey(assets, asset.Key)
		if index >= 0 {
			if assets[index].RootPath != asset.RootPath {
				return contentUploadError{
					Status:  http.StatusBadRequest,
					Message: "Archive contains multiple conflicting folders for " + asset.Key + ".",
				}
			}
			return nil
		}

		assets = append(assets, asset)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return assets, nil
}

func detectFilesystemAsset(kind string, relPath string) (detectedArchiveAsset, bool, error) {
	cleaned, skip, err := normalizeArchivePath(relPath)
	if err != nil || skip {
		return detectedArchiveAsset{}, false, err
	}

	segments := strings.Split(cleaned, "/")
	start := 0
	folder := contentFolderForMessage(kind)
	if idx := indexSegmentSequence(segments, []string{"content", folder}); idx >= 0 {
		start = idx + 2
	} else if idx := indexSegmentSequence(segments, []string{folder}); idx >= 0 {
		start = idx + 1
	}

	trimmedSegments := segments[start:]
	if len(trimmedSegments) == 0 {
		return detectedArchiveAsset{}, false, nil
	}

	rootPath, assetKey, ok := detectArchiveAssetRoot(strings.Join(trimmedSegments, "/"), kind)
	if !ok {
		return detectedArchiveAsset{}, false, nil
	}

	rootSegments := append([]string{}, segments[:start]...)
	if rootPath != "" {
		rootSegments = append(rootSegments, strings.Split(rootPath, "/")...)
	}

	return detectedArchiveAsset{
		Key:      assetKey,
		RootPath: strings.Join(rootSegments, "/"),
	}, true, nil
}

func countFilesInTree(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
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
