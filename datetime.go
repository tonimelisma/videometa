package videometa

import (
	"fmt"
	"strings"
	"time"
)

// parseVideoMetadataTimeValue parses the supported video-metadata time shapes.
func parseVideoMetadataTimeValue(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return time.Time{}, fmt.Errorf("zero time")
		}
		return t, nil
	case string:
		return parseVideoMetadataTimeString(t)
	default:
		return time.Time{}, fmt.Errorf("unsupported type %T for date/time", v)
	}
}

// parseVideoMetadataTimeString parses the date/time formats used by validated
// QuickTime and vendor container metadata.
func parseVideoMetadataTimeString(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000:00:00 00:00:00" {
		return time.Time{}, fmt.Errorf("empty or zero date")
	}

	formats := []string{
		"2006:01:02 15:04:05",
		"2006:01:02 15:04:05-07:00",
		"2006:01:02 15:04:05Z07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02",
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
}

// formatVideoMetadataTimeForExiftool normalizes a supported video metadata
// timestamp into exiftool's QuickTime-style print format.
func formatVideoMetadataTimeForExiftool(s string) string {
	t, err := parseVideoMetadataTimeString(s)
	if err != nil {
		return ""
	}
	_, offset := t.Zone()
	if offset == 0 && t.Location() == time.UTC {
		return t.Format("2006:01:02 15:04:05")
	}
	return t.Format("2006:01:02 15:04:05-07:00")
}
