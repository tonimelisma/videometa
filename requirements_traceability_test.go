package videometa

import (
	"os"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

type traceabilityRow struct {
	requirement string
	testFile    string
	status      string
}

// Validates: REQ-EXIF-04, REQ-EXIF-06, REQ-EXIF-07, REQ-EXIF-08, REQ-EXIF-09, REQ-XMP-04
func TestTraceabilityEvidenceStatuses(t *testing.T) {
	c := qt.New(t)

	rows := loadTraceabilityRows(c)
	expected := map[string]traceabilityRow{
		"REQ-EXIF-04": {
			requirement: "REQ-EXIF-04",
			testFile:    "exif_fields_reference_test.go",
			status:      "Validated",
		},
		"REQ-EXIF-06": {
			requirement: "REQ-EXIF-06",
			testFile:    "videometa_test.go, videometa_meta_items_test.go, videometa_oracle_test.go",
			status:      "Validated (meta/iloc); Covered (UUID)",
		},
		"REQ-EXIF-07": {
			requirement: "REQ-EXIF-07",
			testFile:    "metadecoder_makernotes_test.go",
			status:      "Covered",
		},
		"REQ-EXIF-08": {
			requirement: "REQ-EXIF-08",
			testFile:    "metadecoder_makernotes_test.go, videometa_oracle_test.go",
			status:      "Covered",
		},
		"REQ-EXIF-09": {
			requirement: "REQ-EXIF-09",
			testFile:    "metadecoder_makernotes_test.go",
			status:      "Covered",
		},
		"REQ-XMP-04": {
			requirement: "REQ-XMP-04",
			testFile:    "videometa_golden_test.go, videometa_meta_items_test.go, videometa_oracle_test.go",
			status:      "Validated",
		},
	}

	for requirement, want := range expected {
		row, ok := rows[requirement]
		c.Assert(ok, qt.IsTrue, qt.Commentf("missing traceability row for %s", requirement))
		c.Assert(row.testFile, qt.Equals, want.testFile)
		c.Assert(row.status, qt.Equals, want.status)
	}
}

func loadTraceabilityRows(c *qt.C) map[string]traceabilityRow {
	c.Helper()

	data, err := os.ReadFile("docs/REQUIREMENTS.md")
	c.Assert(err, qt.IsNil)

	rows := make(map[string]traceabilityRow)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| REQ-") {
			continue
		}

		fields := splitMarkdownRow(line)
		if len(fields) < 5 {
			continue
		}
		rows[fields[0]] = traceabilityRow{
			requirement: fields[0],
			testFile:    fields[3],
			status:      fields[4],
		}
	}
	return rows
}

func splitMarkdownRow(line string) []string {
	parts := strings.Split(line, "|")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields = append(fields, part)
	}
	return fields
}
