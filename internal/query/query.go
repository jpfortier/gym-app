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

// QueryParams defines the Query DSL for fetching exercise history with scopes and metrics.
type QueryParams struct {
	Category  string // required, e.g. "bench press"
	Variant   string // default "standard"
	Scope     string // most_recent, recent, best, aggregate, session_detail, trend
	Metric    string // max_weight, latest_weight, max_reps, count_sets, count_sessions, total_volume, estimated_1rm
	FromDate  string // YYYY-MM-DD, optional
	ToDate    string // YYYY-MM-DD, optional
	Limit     int    // default 20, max 50
}

// QueryResult is the structured result from Query().
type QueryResult struct {
	ExerciseName string
	VariantName  string
	Entries      []HistoryEntry
	Metric       string   // e.g. "max_weight"
	Value        *float64 // for metric queries
	CountSets    int      // for aggregate
	CountSessions int    // for aggregate
	TotalVolume  float64 // for aggregate
}
