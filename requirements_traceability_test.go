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

// Validates: REQ-EXIF-04, REQ-EXIF-06, REQ-XMP-04
func TestTraceabilityEvidenceStatuses(t *testing.T) {
	c := qt.New(t)

	rows := loadTraceabilityRows(c)
	expected := map[string]traceabilityRow{
		"REQ-EXIF-04": {
			requirement: "REQ-EXIF-04",
			testFile:    "exif_fields_reference_test.go",
			status:      "Implemented",
		},
		"REQ-EXIF-06": {
			requirement: "REQ-EXIF-06",
			testFile:    "videometa_test.go, videometa_meta_items_test.go, videometa_oracle_test.go",
			status:      "Implemented",
		},
		"REQ-XMP-04": {
			requirement: "REQ-XMP-04",
			testFile:    "videometa_golden_test.go, videometa_meta_items_test.go, videometa_oracle_test.go",
			status:      "Validated (`udta/XMP_` real files); Implemented (UUID, meta/iloc)",
		},
	}

	for requirement, want := range expected {
		row, ok := rows[requirement]
		c.Assert(ok, qt.IsTrue, qt.Commentf("missing traceability row for %s", requirement))
		c.Assert(row.testFile, qt.Equals, want.testFile)
		c.Assert(row.status, qt.Equals, want.status)
	}
}

// Validates: REQ-API-01, REQ-NF-04
func TestValidatedRequirementsUseRealFixtureTests(t *testing.T) {
	c := qt.New(t)

	rows := loadTraceabilityRows(c)
	realFixtureTests := map[string]bool{
		"videometa_test.go":         true,
		"videometa_golden_test.go":  true,
		"videometa_latency_test.go": true,
	}

	var offenders []string
	for _, row := range rows {
		if !strings.Contains(row.status, "Validated") {
			continue
		}
		files := splitCommaSeparatedList(row.testFile)
		hasRealFixtureEvidence := false
		for _, file := range files {
			if realFixtureTests[file] {
				hasRealFixtureEvidence = true
				break
			}
		}
		if !hasRealFixtureEvidence {
			offenders = append(offenders, row.requirement+" -> "+row.testFile+" ["+row.status+"]")
		}
	}

	c.Assert(offenders, qt.HasLen, 0, qt.Commentf("validated rows without real-fixture tests:\n%s", strings.Join(offenders, "\n")))
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

func splitCommaSeparatedList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "—" {
			continue
		}
		result = append(result, part)
	}
	return result
}
