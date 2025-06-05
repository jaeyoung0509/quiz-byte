package util

import (
	"database/sql"
	"time"
)

// StringToNullString converts a string to sql.NullString.
// An empty string is treated as NULL.
func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{} // Valid is false, String is ""
	}
	return sql.NullString{String: s, Valid: true}
}

// TimeToNullTime converts a time.Time to sql.NullTime.
// A zero time is treated as NULL.
func TimeToNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{} // Valid is false, Time is zero value
	}
	return sql.NullTime{Time: t, Valid: true}
}
