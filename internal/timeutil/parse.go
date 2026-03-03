package timeutil

import (
	"fmt"
	"time"
)

// Layouts tried in order when parsing ISO-8601 timestamps.
var layouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
}

// Parse tries to parse an ISO-8601 timestamp string.
func Parse(ts string) (time.Time, error) {
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", ts)
}
