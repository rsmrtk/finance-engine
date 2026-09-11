// Package pgutil converts between pgx/pgtype wire types and the plain Go
// types (uuid.UUID, string, time.Time) used everywhere outside the
// repository layer.
package pgutil

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func UUIDFromGoogle(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func UUIDToGoogle(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

// NullUUIDFromGoogle returns an invalid pgtype.UUID for uuid.Nil, and a
// valid one otherwise. Use for optional foreign keys like category_id.
func NullUUIDFromGoogle(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return UUIDFromGoogle(id)
}

func TextFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func TimeFromGo(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// NullTimeFromGo returns an invalid (NULL) timestamp for a zero time.Time.
// Use for optional date-range filters.
func NullTimeFromGo(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return TimeFromGo(t)
}

func NumericFromString(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

func NumericToString(n pgtype.Numeric) string {
	v, err := n.Value()
	if err != nil || v == nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "0"
}

// NumericFromFloat64 goes through a string representation because
// pgtype.Numeric.Scan only accepts the driver-facing types (string, []byte),
// not a raw float64.
func NumericFromFloat64(f float64) (pgtype.Numeric, error) {
	return NumericFromString(strconv.FormatFloat(f, 'f', -1, 64))
}

func NumericToFloat64(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
