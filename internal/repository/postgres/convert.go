package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================
// pgtype → domain conversions (shared helpers for sqlc adapters)
// ============================================================

// String returns the underlying string when valid, "" otherwise.
func pgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// StringPtr returns &t.String when valid, nil otherwise.
func pgTextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// StringPtrFromText returns &t.String when non-empty, nil otherwise.
func pgTextToStringPtrNonEmpty(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	return &t.String
}

// pgUUIDToUUID converts pgtype.UUID → uuid.UUID (empty when invalid).
func pgUUIDToUUID(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.Nil
	}
	id, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return uuid.Nil
	}
	return id
}

// pgUUIDPtrToUUIDPtr converts pgtype.UUID → *uuid.UUID (nil when invalid).
func pgUUIDPtrToUUIDPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return nil
	}
	return &id
}

// uuidToPgUUID converts *uuid.UUID → pgtype.UUID.
func uuidToPgUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: [16]byte(*u), Valid: true}
}

// uuidToPgUUIDValue converts uuid.UUID → pgtype.UUID.
func uuidToPgUUIDValue(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: [16]byte(u), Valid: true}
}

// pgTimestamptzToPtr converts pgtype.Timestamptz → *time.Time.
func pgTimestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// pgTimestamptzToTime converts pgtype.Timestamptz → time.Time (zero value when invalid).
func pgTimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// pgNumericToFloat64 converts pgtype.Numeric → float64.
func pgNumericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, _ := n.Float64Value()
	return v.Float64
}

// pgInt4ToInt converts pgtype.Int4 → int (0 when invalid).
func pgInt4ToInt(n pgtype.Int4) int {
	if !n.Valid {
		return 0
	}
	return int(n.Int32)
}
