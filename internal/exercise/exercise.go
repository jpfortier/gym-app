package exercise

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	Name       string
	ShowWeight bool
	ShowReps   bool
	CreatedAt  time.Time
}

type Variant struct {
	ID         uuid.UUID
	CategoryID uuid.UUID
	UserID     *uuid.UUID
	Name       string
	Standard   bool
	CreatedAt  time.Time
}
