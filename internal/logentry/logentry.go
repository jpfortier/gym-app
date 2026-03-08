package logentry

import (
	"time"

	"github.com/google/uuid"
)

type LogEntry struct {
	ID                 uuid.UUID
	SessionID          uuid.UUID
	ExerciseVariantID  uuid.UUID
	RawSpeech          string
	Notes              string
	DisabledAt         *time.Time
	CreatedAt          time.Time
	Sets               []LogEntrySet
}

type LogEntrySet struct {
	ID         uuid.UUID
	LogEntryID uuid.UUID
	Weight     *float64 // nil for bodyweight
	Reps       int
	SetOrder   int
	SetType    string
	CreatedAt  time.Time
}

type SetInput struct {
	Weight   *float64
	Reps     int
	SetOrder int
	SetType  string
}
