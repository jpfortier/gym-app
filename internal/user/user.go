package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	GoogleID  string
	Email     string
	Name      string
	PhotoURL  string
	Role      string // user, coach, owner, admin
	CreatedAt time.Time
}
