package videometa

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

type traceabilityRow struct {
	requirement string
	testFile    string
	status      string
}

type traceabilityEvidence struct {
	file        string
	funcName    string
	realFixture bool
}

// Validates: REQ-API-01
func TestRequirementsDeclareEmbeddedImageMetadataNonGoal(t *testing.T) {
	c := qt.New(t)

	data, err := os.ReadFile("docs/REQUIREMENTS.md")
	c.Assert(err, qt.IsNil)

	text := string(data)
	c.Assert(strings.Contains(text, "embedded image metadata"), qt.IsTrue)
	c.Assert(strings.Contains(text, "EXIF/TIFF"), qt.IsTrue)
	c.Assert(strings.Contains(text, "XMP/RDF"), qt.IsTrue)
	c.Assert(strings.Contains(text, "IPTC-IIM"), qt.IsTrue)
}

// Validates: REQ-API-01, REQ-NF-04
func TestTraceabilityRowsReferenceExistingValidatingTests(t *testing.T) {
	c := qt.New(t)

	rows := loadTraceabilityRows(c)
	allowedStatuses := []string{"Validated", "Implemented", "Static", "Config", "Pending"}
	var offenders []string
	for _, row := range rows {
		statusOK := false
		for _, status := range allowedStatuses {
			if strings.Contains(row.status, status) {
				statusOK = true
				break
			}
		}
		if !statusOK {
			offenders = append(offenders, row.requirement+" has unsupported status "+row.status)
			continue
		}

		files := splitCommaSeparatedList(row.testFile)
		if len(files) == 0 {
			continue
		}

		for _, file := range files {
			if _, err := os.Stat(file); err != nil {
				offenders = append(offenders, row.requirement+" cites missing test file "+file)
				continue
			}

			evidence := collectTraceabilityEvidence(c, file)
			if len(evidence[row.requirement]) == 0 {
				offenders = append(offenders, row.requirement+" cites "+file+" but the file has no // Validates comment for it")
			}
		}
	}

	c.Assert(offenders, qt.HasLen, 0, qt.Commentf("traceability matrix issues:\n%s", strings.Join(offenders, "\n")))
}

// Validates: REQ-API-01, REQ-NF-04
func TestValidatedRequirementsUseRealFixtureEvidence(t *testing.T) {
	c := qt.New(t)

	rows := loadTraceabilityRows(c)
	var offenders []string

	for _, row := range rows {
		if !strings.Contains(row.status, "Validated") {
			continue
		}

		files := splitCommaSeparatedList(row.testFile)
		hasRealFixtureEvidence := false
		for _, file := range files {
			for _, evidence := range collectTraceabilityEvidence(c, file)[row.requirement] {
				if evidence.realFixture {
					hasRealFixtureEvidence = true
					break
				}
			}
			if hasRealFixtureEvidence {
				break
			}
		}

		if !hasRealFixtureEvidence {
			offenders = append(offenders, row.requirement+" -> "+row.testFile+" ["+row.status+"]")
		}
	}

	c.Assert(offenders, qt.HasLen, 0, qt.Commentf("validated rows without real-fixture evidence:\n%s", strings.Join(offenders, "\n")))
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

func collectTraceabilityEvidence(c *qt.C, file string) map[string][]traceabilityEvidence {
	c.Helper()

	data, err := os.ReadFile(file)
	c.Assert(err, qt.IsNil)

	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	c.Assert(err, qt.IsNil)

	evidenceByRequirement := make(map[string][]traceabilityEvidence)
	for _, decl := range parsedFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil || fn.Body == nil {
			continue
		}

		requirements := validatedRequirementsFromComment(fn.Doc.Text())
		if len(requirements) == 0 {
			continue
		}

		start := fset.Position(fn.Body.Pos()).Offset
		end := fset.Position(fn.Body.End()).Offset
		body := string(data[start:end])
		realFixture := strings.Contains(body, "testdata/")

		for _, requirement := range requirements {
			evidenceByRequirement[requirement] = append(evidenceByRequirement[requirement], traceabilityEvidence{
				file:        filepath.Base(file),
				funcName:    fn.Name.Name,
				realFixture: realFixture,
			})
		}
	}

	return evidenceByRequirement
}

func validatedRequirementsFromComment(commentText string) []string {
	lines := strings.Split(commentText, "\n")
	var requirements []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Validates:") {
			continue
		}
		for _, requirement := range strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Validates:")), ",") {
			requirement = strings.TrimSpace(requirement)
			if requirement != "" {
				requirements = append(requirements, requirement)
			}
		}
	}
	return requirements
}
