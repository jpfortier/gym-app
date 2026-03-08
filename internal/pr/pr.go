package pr

import (
	"time"

	"github.com/google/uuid"
)

type PersonalRecord struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	ExerciseVariantID uuid.UUID
	PRType            string // natural_set, one_rep_max, etc.
	Weight            float64
	Reps              *int
	LogEntrySetID     *uuid.UUID
	ImageURL          string
	CreatedAt         time.Time
}
