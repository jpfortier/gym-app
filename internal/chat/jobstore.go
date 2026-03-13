package chat

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const jobTTL = 10 * time.Minute

// JobStatus is the state of an async chat job.
type JobStatus string

const (
	JobStatusProcessing JobStatus = "processing"
	JobStatusComplete   JobStatus = "complete"
	JobStatusFailed     JobStatus = "failed"
)

// Job holds state for an async chat request.
type Job struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Text      string
	Status    JobStatus
	Result    *Response
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// JobStore holds in-memory chat jobs. Safe for concurrent use.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*Job
}

// NewJobStore returns a new job store. Call RunCleanup to start TTL cleanup.
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[uuid.UUID]*Job)}
}

// Create creates a job and returns its ID. Caller must set Text and Status before starting LLM.
func (s *JobStore) Create(userID uuid.UUID, text string) uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.Must(uuid.NewV7())
	now := time.Now()
	s.jobs[id] = &Job{
		ID:        id,
		UserID:    userID,
		Text:      text,
		Status:    JobStatusProcessing,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return id
}

// Get returns the job if it exists and belongs to the user.
func (s *JobStore) Get(id uuid.UUID, userID uuid.UUID) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok || j.UserID != userID {
		return nil, false
	}
	return j, true
}

// Complete sets the job as complete with the given result.
func (s *JobStore) Complete(id uuid.UUID, result *Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobStatusComplete
		j.Result = result
		j.UpdatedAt = time.Now()
	}
}

// Fail sets the job as failed with the given error.
func (s *JobStore) Fail(id uuid.UUID, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobStatusFailed
		j.Error = errMsg
		j.UpdatedAt = time.Now()
	}
}

// RunCleanup removes jobs older than jobTTL. Call in a goroutine.
func (s *JobStore) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *JobStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-jobTTL)
	for id, j := range s.jobs {
		if j.UpdatedAt.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
}

