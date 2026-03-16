package timeutil

import (
	"fmt"
	"time"
)

// Layouts tried in order when parsing ISO-8601 timestamps.
// RFC3339Nano matches all RFC3339 strings so RFC3339 is omitted.
var layouts = []string{
	time.RFC3339Nano,
}

// localLayouts are parsed in the local timezone rather than UTC.
var localLayouts = []string{
	"2006-01-02T15:04:05",
}

// Parse tries to parse an ISO-8601 timestamp string.
func Parse(ts string) (time.Time, error) {
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}
	for _, layout := range localLayouts {
		if t, err := time.ParseInLocation(layout, ts, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", ts)
}
