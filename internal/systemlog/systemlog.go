package systemlog

import (
	"time"

	"github.com/google/uuid"
)

// Category for system log entries.
const (
	CategoryAuthSuccess = "auth_success"
	CategoryAuthFailure = "auth_failure"
	CategoryAction      = "action"
	CategoryAI          = "ai"
	CategoryException   = "exception"
)

// Entry represents a persisted system log record.
type Entry struct {
	ID        uuid.UUID
	CreatedAt time.Time
	Category  string
	UserID    *uuid.UUID
	Method    string
	Path      string
	Details   map[string]interface{}
	Error     string
}

// InsertParams holds fields for inserting a log entry.
type InsertParams struct {
	Category string
	UserID   *uuid.UUID
	Method   string
	Path     string
	Details  map[string]interface{}
	Error    string
}

// ListParams holds optional filters for listing logs.
type ListParams struct {
	Category string
	UserID   *uuid.UUID
	Limit    int
	Offset   int
}
