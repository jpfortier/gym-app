package db

import (
	"database/sql"

	"github.com/google/uuid"
)

// NullStr returns nil for empty string, for use in INSERT/UPDATE.
func NullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// NullFloat64 returns nil for nil pointer, for use in INSERT/UPDATE.
func NullFloat64(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}

// NullInt returns nil for nil pointer, for use in INSERT/UPDATE.
func NullInt(i *int) interface{} {
	if i == nil {
		return nil
	}
	return *i
}

// NullUUID returns nil for nil pointer, for use in INSERT/UPDATE.
func NullUUID(u *uuid.UUID) interface{} {
	if u == nil {
		return nil
	}
	return *u
}

// EnsureV7 sets id to a new V7 UUID if it is uuid.Nil.
func EnsureV7(id *uuid.UUID) {
	if *id == uuid.Nil {
		*id = uuid.Must(uuid.NewV7())
	}
}

// NullStringToUUIDPtr converts sql.NullString to *uuid.UUID for scanning.
func NullStringToUUIDPtr(n sql.NullString) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	u, _ := uuid.Parse(n.String)
	return &u
}

// NullInt64ToIntPtr converts sql.NullInt64 to *int for scanning.
func NullInt64ToIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	i := int(n.Int64)
	return &i
}
