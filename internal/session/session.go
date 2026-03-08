package session

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Date      time.Time // date only (time truncated)
	CreatedAt time.Time
}
