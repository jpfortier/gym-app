package query

import "time"

// HistoryEntry represents one log entry in query history.
type HistoryEntry struct {
	SessionDate string
	RawSpeech   string
	Sets        []SetSummary
	CreatedAt   time.Time
}

type SetSummary struct {
	Weight  *float64
	Reps    int
	SetType string
}
