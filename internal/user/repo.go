package user

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) GetByGoogleID(ctx context.Context, googleID string) (*User, error) {
	var u User
	var email, name, photoURL sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, google_id, email, name, photo_url, role, created_at
		 FROM users WHERE google_id = $1`,
		googleID,
	).Scan(&u.ID, &u.GoogleID, &email, &name, &photoURL, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Email = email.String
	u.Name = name.String
	u.PhotoURL = photoURL.String
	return &u, nil
}

func (r *Repo) Create(ctx context.Context, u *User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.Must(uuid.NewV7())
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO users (id, google_id, email, name, photo_url)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		u.ID, u.GoogleID, u.Email, u.Name, u.PhotoURL,
	).Scan(&u.ID, &u.CreatedAt)
}
