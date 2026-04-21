package main

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

const contentJobRetention = 10 * time.Minute
const contentJobRecentWindow = 2 * time.Minute

type ContentJob struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Source         string   `json:"source"`
	SourceName     string   `json:"source_name"`
	SourceURL      string   `json:"source_url,omitempty"`
	Status         string   `json:"status"`
	Phase          string   `json:"phase"`
	Message        string   `json:"message"`
	Progress       int      `json:"progress"`
	Downloaded     int64    `json:"downloaded_bytes"`
	DownloadTotal  int64    `json:"download_total_bytes"`
	FilesWritten   int      `json:"files_written"`
	ImportedAssets []string `json:"imported_assets,omitempty"`
	TracksTotal    int      `json:"tracks_total"`
	CarsTotal      int      `json:"cars_total"`
	WeathersTotal  int      `json:"weathers_total"`
	StartedAt      int64    `json:"started_at"`
	UpdatedAt      int64    `json:"updated_at"`
	FinishedAt     int64    `json:"finished_at"`
}

type ContentJobStore struct {
	mutex sync.RWMutex
	jobs  map[string]*ContentJob
}

func NewContentJobStore() *ContentJobStore {
	return &ContentJobStore{
		jobs: map[string]*ContentJob{},
	}
}

func (s *ContentJobStore) Create(kind string, source string, sourceName string, sourceURL string) ContentJob {
	now := time.Now().Unix()
	job := &ContentJob{
		ID:         newContentJobID(),
		Kind:       kind,
		Source:     source,
		SourceName: sourceName,
		SourceURL:  sourceURL,
		Status:     "queued",
		Phase:      "queued",
		Message:    "Waiting to start.",
		Progress:   0,
		StartedAt:  now,
		UpdatedAt:  now,
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.pruneLocked()
	s.jobs[job.ID] = job
	return cloneContentJob(job)
}

func (s *ContentJobStore) Update(id string, mutator func(job *ContentJob)) (ContentJob, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return ContentJob{}, false
	}

	mutator(job)
	job.UpdatedAt = time.Now().Unix()
	return cloneContentJob(job), true
}

func (s *ContentJobStore) Get(id string) (ContentJob, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return ContentJob{}, false
	}

	return cloneContentJob(job), true
}

func (s *ContentJobStore) ListVisible() []ContentJob {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pruneLocked()

	now := time.Now()
	list := make([]ContentJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.FinishedAt == 0 || now.Sub(time.Unix(job.FinishedAt, 0)) <= contentJobRecentWindow {
			list = append(list, cloneContentJob(job))
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].UpdatedAt == list[j].UpdatedAt {
			return list[i].StartedAt > list[j].StartedAt
		}
		return list[i].UpdatedAt > list[j].UpdatedAt
	})

	return list
}

func (s *ContentJobStore) pruneLocked() {
	now := time.Now()
	for id, job := range s.jobs {
		if job.FinishedAt == 0 {
			continue
		}
		if now.Sub(time.Unix(job.FinishedAt, 0)) > contentJobRetention {
			delete(s.jobs, id)
		}
	}
}

func cloneContentJob(job *ContentJob) ContentJob {
	clone := *job
	if job.ImportedAssets != nil {
		clone.ImportedAssets = append([]string{}, job.ImportedAssets...)
	}
	return clone
}

func newContentJobID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405")))
	}
	return hex.EncodeToString(buf)
}
