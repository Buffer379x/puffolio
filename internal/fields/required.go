package fields

import (
	"fmt"
	"time"
)

// RequiredForRecord evaluates a persisted record's cutoff chronologically. Event
// timestamps can have different valid RFC3339 fractional precision or offsets.
func RequiredForRecord(d Definition, createdAt string) (bool, error) {
	if !d.Required || d.Archived {
		return false, nil
	}
	if d.RequiredFrom == "" {
		return true, nil
	}
	created, e := time.Parse(time.RFC3339Nano, createdAt)
	if e != nil {
		return false, fmt.Errorf("Ungültiger Erstellungszeitpunkt")
	}
	cutoff, e := time.Parse(time.RFC3339Nano, d.RequiredFrom)
	if e != nil {
		return false, fmt.Errorf("Ungültige Pflichtfeldregel")
	}
	return !created.Before(cutoff), nil
}
