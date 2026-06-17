package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"runtime"
	"sync"

	"golang.org/x/image/draw"
)

// Width every cached preview is downscaled to (matches the value used when
// building smcontent.zip). Images already narrower are re-encoded at native
// size — the recompression alone shrinks them.
const compressMaxWidth = 640

const compressJPEGQuality = 82

// scaleImage downscales img to maxWidth (preserving aspect ratio) using scaler;
// images already at/under maxWidth are returned unchanged.
func scaleImage(img image.Image, maxWidth int, scaler draw.Scaler) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxWidth || w == 0 {
		return img
	}
	nh := h * maxWidth / w
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, nh))
	scaler.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// compressToJPEG re-encodes opaque preview photos (car + track previews) as a
// downscaled JPEG.
func compressToJPEG(src []byte, maxWidth int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, scaleImage(img, maxWidth, draw.CatmullRom), &jpeg.Options{Quality: compressJPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// compressToPNG re-encodes images that must keep their alpha channel (track
// outline/map silhouettes, drawn as a white overlay) as a downscaled PNG.
func compressToPNG(src []byte, maxWidth int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, scaleImage(img, maxWidth, draw.CatmullRom)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// loadSourceImage returns the first existing source image, preferring the
// full-quality file on disk and falling back to smcontent.zip.
func loadSourceImage(diskCandidates [][]string, zipCandidates []string) ([]byte, bool) {
	if data, ok := readContentDiskFile(diskCandidates); ok {
		return data, true
	}
	for _, zp := range zipCandidates {
		if data, ok := readZipFile(zp); ok {
			return data, true
		}
	}
	return nil, false
}

type compressStats struct {
	Images   int   `json:"images"`
	SrcBytes int64 `json:"src_bytes"`
	OutBytes int64 `json:"out_bytes"`
}

// imageJob describes one image to compress and where to find its source.
type imageJob struct {
	kind    string
	key     string
	config  string
	variant string
	disk    [][]string // candidate paths under <install>/content
	zip     []string   // candidate paths inside smcontent.zip
	png     bool       // keep alpha (PNG) vs re-encode as JPEG
}

// carImageJobs returns the compression jobs for one car (one preview per skin).
func carImageJobs(car CacheCar) []imageJob {
	if car.Key == nil {
		return nil
	}
	ck := *car.Key
	jobs := make([]imageJob, 0, len(car.Skins))
	for _, skin := range car.Skins {
		jobs = append(jobs, imageJob{
			kind: "car", key: ck, config: skin.Key, variant: "preview",
			disk: [][]string{
				{"cars", ck, "skins", skin.Key, "preview.jpg"},
				{"cars", ck, "skins", skin.Key, "preview.png"},
				{"cars", ck, "skins", skin.Key, "livery.png"},
			},
			zip: []string{"cars/" + ck + "/skins/" + skin.Key + "/preview.jpg"},
		})
	}
	return jobs
}

// trackImageJobs returns the compression jobs for one track layout row
// (preview + outline + map). outline/map keep alpha and are stored as PNG.
func trackImageJobs(t CacheTrack) []imageJob {
	if t.Key == nil {
		return nil
	}
	tk := *t.Key
	cfg := ""
	if t.Config != nil {
		cfg = *t.Config
	}
	if cfg != "" {
		return []imageJob{
			{kind: "track", key: tk, config: cfg, variant: "preview",
				disk: [][]string{{"tracks", tk, "ui", cfg, "preview.png"}},
				zip:  []string{"tracks/" + tk + "/" + cfg + "/preview.png"}},
			{kind: "track", key: tk, config: cfg, variant: "outline", png: true,
				disk: [][]string{{"tracks", tk, "ui", cfg, "outline.png"}},
				zip:  []string{"tracks/" + tk + "/" + cfg + "/outline.png"}},
			{kind: "track", key: tk, config: cfg, variant: "map", png: true,
				disk: [][]string{{"tracks", tk, cfg, "map.png"}},
				zip:  []string{"tracks/" + tk + "/" + cfg + "/map.png"}},
		}
	}
	return []imageJob{
		{kind: "track", key: tk, config: "", variant: "preview",
			disk: [][]string{{"tracks", tk, "ui", "preview.png"}},
			zip:  []string{"tracks/" + tk + "/ui/preview.png"}},
		{kind: "track", key: tk, config: "", variant: "outline", png: true,
			disk: [][]string{{"tracks", tk, "ui", "outline.png"}},
			zip:  []string{"tracks/" + tk + "/ui/outline.png"}},
		{kind: "track", key: tk, config: "", variant: "map", png: true,
			disk: [][]string{{"tracks", tk, "map.png"}},
			zip:  []string{"tracks/" + tk + "/map.png"}},
	}
}

// compressAllContentImages compresses the previews of every cached car and
// track. Used by the manual "Compress images" action and on recache.
func compressAllContentImages(dba Dbaccess) (compressStats, error) {
	cars, err := dba.selectCacheCars()
	if err != nil {
		return compressStats{}, err
	}
	tracks, err := dba.selectCacheTracks()
	if err != nil {
		return compressStats{}, err
	}

	jobs := make([]imageJob, 0, len(cars)+len(tracks)*3)
	for _, car := range cars {
		jobs = append(jobs, carImageJobs(car)...)
	}
	for _, t := range tracks {
		jobs = append(jobs, trackImageJobs(t)...)
	}
	return runImageJobs(dba, jobs), nil
}

// compressContentImagesForKeys compresses only the previews of the named
// content items, used to auto-compress freshly imported assets without
// re-processing the whole library. kind is "car" or "track" (singular).
func compressContentImagesForKeys(dba Dbaccess, kind string, keys []string) (compressStats, error) {
	if len(keys) == 0 {
		return compressStats{}, nil
	}
	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[k] = true
	}

	var jobs []imageJob
	switch kind {
	case "car":
		cars, err := dba.selectCacheCars()
		if err != nil {
			return compressStats{}, err
		}
		for _, car := range cars {
			if car.Key != nil && wanted[*car.Key] {
				jobs = append(jobs, carImageJobs(car)...)
			}
		}
	case "track":
		tracks, err := dba.selectCacheTracks()
		if err != nil {
			return compressStats{}, err
		}
		for _, t := range tracks {
			if t.Key != nil && wanted[*t.Key] {
				jobs = append(jobs, trackImageJobs(t)...)
			}
		}
	}
	return runImageJobs(dba, jobs), nil
}

// runImageJobs decodes/resizes/encodes the jobs in parallel (CPU-bound) and
// stores each result, serialising the SQLite writes and stats under a mutex.
// Missing sources and per-image failures are skipped, not fatal.
func runImageJobs(dba Dbaccess, jobs []imageJob) compressStats {
	var stats compressStats
	var mu sync.Mutex
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j imageJob) {
			defer wg.Done()
			defer func() { <-sem }()

			src, ok := loadSourceImage(j.disk, j.zip)
			if !ok {
				return
			}

			var out []byte
			var ct string
			var err error
			if j.png {
				out, err = compressToPNG(src, compressMaxWidth)
				ct = "image/png"
			} else {
				out, err = compressToJPEG(src, compressMaxWidth)
				ct = "image/jpeg"
			}
			if err != nil {
				log.Printf("Could not compress image %s/%s/%s/%s: %v", j.kind, j.key, j.config, j.variant, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if err := dba.upsertCacheImage(j.kind, j.key, j.config, j.variant, ct, out); err != nil {
				log.Printf("Could not store compressed image %s/%s/%s/%s: %v", j.kind, j.key, j.config, j.variant, err)
				return
			}
			stats.Images++
			stats.SrcBytes += int64(len(src))
			stats.OutBytes += int64(len(out))
		}(j)
	}
	wg.Wait()
	return stats
}
