package videometa

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

type decodeReaderFactory func(data []byte) io.Reader

// Validates: REQ-EXIF-06, REQ-NF-04
func TestOracleMetaIlocEXIFFileOffset(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Oracle Offset Cam"))
	data := buildMP4WithMetaFileItem(itemPayload, buildInfeEXIF(1), buildIlocFileOffset(1, 0, uint32(len(itemPayload))))

	testTempOracleExhaustive(t, c, "oracle-iloc-offset.mp4", data, EXIF, []string{"EXIF"}, nil)
}

// Validates: REQ-EXIF-06, REQ-NF-04
func TestOracleMetaIlocEXIFIDAT(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x0110, "Oracle IDAT Model"))
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	testTempOracleExhaustive(t, c, "oracle-iloc-idat.mp4", data, EXIF, []string{"EXIF"}, nil)
}

// Validates: REQ-EXIF-06, REQ-NF-04
func TestOracleMetaIlocEXIFIDATNonSeekable(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Oracle Reader Cam"))
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	testTempOracleExhaustive(t, c, "oracle-iloc-idat-reader.mp4", data, EXIF, []string{"EXIF"}, func(payload []byte) io.Reader {
		return readerOnly{readerSeekerFromBytes(payload)}
	})
}

// Validates: REQ-XMP-04, REQ-NF-04
func TestOracleMetaIlocXMPIDAT(t *testing.T) {
	c := qt.New(t)

	itemPayload := buildMinimalXMPPacket("Oracle XMP")
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeXMP(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	testTempOracleExhaustive(t, c, "oracle-iloc-xmp.mp4", data, XMP, []string{"XMP"}, nil)
}

// Validates: REQ-XMP-04, REQ-NF-04
func TestOracleXMPUUID(t *testing.T) {
	c := qt.New(t)

	data := buildMP4WithXMPUUID(buildMinimalXMPPacket("Oracle UUID XMP"))
	testTempOracleExhaustive(t, c, "oracle-xmp-uuid.mp4", data, XMP, []string{"XMP"}, nil)
}

// Validates: REQ-EXIF-06
func TestMetaIlocIDATExtentPastBoxWarnsAndSkips(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Too Long"))
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIlocIDAT(1, uint32(len(itemPayload)+1)))

	tags, warnings := decodeAllWithWarnings(c, data, EXIF)
	_, found := flattenSourceTags(tags.EXIF())["Make"]
	c.Assert(found, qt.IsFalse)
	c.Assert(anyWarningContains(warnings, "exceeds idat box bounds"), qt.IsTrue, qt.Commentf("warnings: %v", warnings))
}

// Validates: REQ-EXIF-06
func TestMetaIlocUnsupportedConstructionMethodWarnsAndSkips(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Unsupported"))
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIloc(1, 2, 0, testIlocExtent{
		offset: 0,
		length: uint32(len(itemPayload)),
	}))

	tags, warnings := decodeAllWithWarnings(c, data, EXIF)
	_, found := flattenSourceTags(tags.EXIF())["Make"]
	c.Assert(found, qt.IsFalse)
	c.Assert(anyWarningContains(warnings, "construction method 2"), qt.IsTrue, qt.Commentf("warnings: %v", warnings))
}

func testTempOracleExhaustive(t *testing.T, c *qt.C, fileName string, data []byte, sources Source, groups []string, readerFactory decodeReaderFactory) {
	t.Helper()

	path := writeTempMedia(c, t, fileName, data)
	var decodeInput io.Reader = readerSeekerFromBytes(data)
	if readerFactory != nil {
		decodeInput = readerFactory(data)
	}

	tags, _, err := decodeAllForTest(Options{
		R:       decodeInput,
		Sources: sources,
	})
	c.Assert(err, qt.IsNil)

	golden := runExiftoolOracle(t, path)
	for _, group := range groups {
		goldenGroup, ok := golden[group]
		c.Assert(ok, qt.IsTrue, qt.Commentf("missing exiftool group %s", group))

		goldenMap, ok := goldenGroup.(map[string]any)
		c.Assert(ok, qt.IsTrue, qt.Commentf("golden %s is not a map", group))

		vmTags := goldenGroupTags(tags, group)
		c.Assert(vmTags != nil, qt.IsTrue, qt.Commentf("no videometa tags for group %s", group))

		compareAllGoldenTags(c, vmTags, goldenMap, group)
		assertExtraTagsNotInGolden(c, vmTags, goldenMap, group)
	}
}

func writeTempMedia(c *qt.C, t *testing.T, fileName string, data []byte) string {
	c.Helper()
	t.Helper()

	path := filepath.Join(t.TempDir(), fileName)
	err := os.WriteFile(path, data, 0o644)
	c.Assert(err, qt.IsNil)
	return path
}

func runExiftoolOracle(t *testing.T, path string) map[string]any {
	t.Helper()

	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not available")
	}

	cmd := exec.Command("exiftool", "-n", "-json", "-g", "--File:all", "--ExifTool:all", path)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run exiftool on %s: %v", path, err)
	}

	var results []map[string]any
	if err := json.Unmarshal(output, &results); err != nil {
		t.Fatalf("decode exiftool json: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("exiftool returned no results for %s", path)
	}
	return results[0]
}

func decodeAllWithWarnings(c *qt.C, data []byte, sources Source) (Tags, []string) {
	c.Helper()

	var warnings []string
	tags, _, err := decodeAllForTest(Options{
		R:       readerSeekerFromBytes(data),
		Sources: sources,
		Warnf: func(format string, args ...any) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		},
	})
	c.Assert(err, qt.IsNil)
	return tags, warnings
}

func anyWarningContains(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
