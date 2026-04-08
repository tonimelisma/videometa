package videometa

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-API-11, REQ-QT-08
func TestParseVideoMetadataTimeString(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name      string
		input     string
		wantYear  int
		wantMonth time.Month
		wantDay   int
		wantHour  int
		wantErr   bool
	}{
		{
			name:      "quicktime offset without colon",
			input:     "2026-03-18T16:39:46-0700",
			wantYear:  2026,
			wantMonth: time.March,
			wantDay:   18,
			wantHour:  16,
		},
		{
			name:      "quicktime exiftool format with colon offset",
			input:     "2026:03:18 16:39:46-07:00",
			wantYear:  2026,
			wantMonth: time.March,
			wantDay:   18,
			wantHour:  16,
		},
		{
			name:      "iso8601 utc",
			input:     "2024-06-15T10:30:00Z",
			wantYear:  2024,
			wantMonth: time.June,
			wantDay:   15,
			wantHour:  10,
		},
		{
			name:    "zero date rejected",
			input:   "0000:00:00 00:00:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			got, err := parseVideoMetadataTimeString(tt.input)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
			c.Assert(got.Year(), qt.Equals, tt.wantYear)
			c.Assert(got.Month(), qt.Equals, tt.wantMonth)
			c.Assert(got.Day(), qt.Equals, tt.wantDay)
			c.Assert(got.Hour(), qt.Equals, tt.wantHour)
		})
	}
}

// Validates: REQ-QT-07, REQ-QT-08
func TestFormatVideoMetadataTimeForExiftool(t *testing.T) {
	c := qt.New(t)

	c.Assert(
		formatVideoMetadataTimeForExiftool("2026-03-18T16:39:46-0700"),
		qt.Equals,
		"2026:03:18 16:39:46-07:00",
	)
	c.Assert(
		formatVideoMetadataTimeForExiftool("2024-06-15T17:30:00Z"),
		qt.Equals,
		"2024:06:15 17:30:00",
	)
	c.Assert(formatVideoMetadataTimeForExiftool("not-a-date"), qt.Equals, "")
}
