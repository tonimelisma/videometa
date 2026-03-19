package videometa

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// --- Exhaustive golden test infrastructure ---

// compareAllGoldenTags asserts that for every tag exiftool emits in a group,
// videometa emits the same tag with the same value.
func compareAllGoldenTags(c *qt.C, tags map[string]TagInfo, golden map[string]any, groupName string) {
	c.Helper()
	for name, goldenVal := range golden {
		// Skip binary data placeholders — exiftool emits "(Binary data N bytes, ...)"
		// for non-extractable binary values. We emit raw bytes.
		if s, ok := goldenVal.(string); ok && len(s) > 12 && s[:12] == "(Binary data" {
			_, exists := tags[name]
			c.Assert(exists, qt.IsTrue, qt.Commentf(
				"videometa missing binary %s tag %q", groupName, name))
			continue
		}
		tag, ok := tags[name]
		c.Assert(ok, qt.IsTrue, qt.Commentf(
			"videometa missing %s tag %q that exiftool emits (exiftool value: %v [%T])",
			groupName, name, goldenVal, goldenVal))
		if ok {
			assertGoldenValue(c, groupName, name, tag.Value, goldenVal)
		}
	}
}

// assertExtraTagsNotInGolden asserts that videometa does not emit tags
// that exiftool doesn't know about.
func assertExtraTagsNotInGolden(c *qt.C, tags map[string]TagInfo, golden map[string]any, groupName string) {
	c.Helper()
	for name := range tags {
		_, ok := golden[name]
		c.Assert(ok, qt.IsTrue, qt.Commentf(
			"videometa emits %s tag %q that exiftool does not (value: %v [%T])",
			groupName, name, tags[name].Value, tags[name].Value))
	}
}

// assertGoldenValue compares a videometa tag value against an exiftool golden value.
func assertGoldenValue(c *qt.C, group, name string, got any, want any) {
	c.Helper()
	ctx := qt.Commentf("%s.%s: got %v (%T), want %v (%T)", group, name, got, got, want, want)

	switch w := want.(type) {
	case float64:
		gotF, ok := toFloat64(got)
		if !ok {
			// Try time.Time → date tags where exiftool emits 0 for Mac epoch.
			if _, isTime := got.(time.Time); isTime {
				gotStr := formatTimeForGolden(got.(time.Time))
				wantStr := fmt.Sprintf("%v", w)
				c.Assert(gotStr, qt.Equals, wantStr, ctx)
				return
			}
			// Try string→float64 (e.g., ContentCreateDate "2010" → 2010).
			if s, isStr := got.(string); isStr {
				gotStr := fmt.Sprintf("%v", w)
				// If golden is an integer (no decimal part), compare as integer string.
				if w == math.Floor(w) {
					gotStr = fmt.Sprintf("%d", int(w))
				}
				c.Assert(s, qt.Equals, gotStr, ctx)
				return
			}
			c.Assert(ok, qt.IsTrue, qt.Commentf("%s.%s: cannot convert %T to float64", group, name, got))
			return
		}
		if w == 0 && gotF == 0 {
			return // Both zero.
		}
		// Relative tolerance for large values, absolute for small.
		if math.Abs(w) > 1 {
			c.Assert(math.Abs((gotF-w)/w) < 0.0001, qt.IsTrue, ctx)
		} else {
			c.Assert(math.Abs(gotF-w) < 0.001, qt.IsTrue, ctx)
		}

	case string:
		gotStr := formatValueForGolden(got)
		// If golden is in exiftool date format and got is ISO 8601, convert for comparison.
		if gotStr != w {
			if converted := convertDateToExiftool(gotStr); converted != "" && converted == w {
				gotStr = converted
			}
		}
		c.Assert(gotStr, qt.Equals, w, ctx)

	case []any:
		gotSlice, ok := got.([]string)
		c.Assert(ok, qt.IsTrue, qt.Commentf("%s.%s: expected []string, got %T", group, name, got))
		c.Assert(len(gotSlice), qt.Equals, len(w), ctx)
		for i, wElem := range w {
			c.Assert(gotSlice[i], qt.Equals, wElem.(string), qt.Commentf("%s.%s[%d]", group, name, i))
		}

	case bool:
		gotBool, ok := got.(bool)
		c.Assert(ok, qt.IsTrue, qt.Commentf("%s.%s: expected bool, got %T", group, name, got))
		c.Assert(gotBool, qt.Equals, w, ctx)

	default:
		c.Assert(formatValueForGolden(got), qt.Equals, fmt.Sprintf("%v", want), ctx)
	}
}

// formatValueForGolden converts a Go value to a string matching exiftool's output format.
func formatValueForGolden(v any) string {
	switch t := v.(type) {
	case time.Time:
		return formatTimeForGolden(t)
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatTimeForGolden formats a time.Time to match exiftool's date format.
// exiftool uses "YYYY:MM:DD HH:MM:SS" for dates without timezone,
// and "YYYY:MM:DD HH:MM:SS+HH:MM" for dates with timezone.
func formatTimeForGolden(t time.Time) string {
	if t.IsZero() {
		return "0000:00:00 00:00:00"
	}
	_, offset := t.Zone()
	if offset == 0 && t.Location() == time.UTC {
		return t.Format("2006:01:02 15:04:05")
	}
	return t.Format("2006:01:02 15:04:05-07:00")
}

// goldenGroupTags returns the videometa tags for a given exiftool group name.
func goldenGroupTags(tags Tags, group string) map[string]TagInfo {
	switch group {
	case "QuickTime":
		return tags.QuickTime()
	case "MakerNotes":
		return tags.MakerNotes()
	case "XMP":
		return tags.XMP()
	case "XML":
		return tags.XML()
	case "Composite":
		return tags.Composite()
	default:
		return nil
	}
}

// testGoldenExhaustive runs an exhaustive golden comparison for all groups.
func testGoldenExhaustive(c *qt.C, videoPath string, goldenPath string, groups []string) {
	c.Helper()

	f, err := os.Open(videoPath)
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	// Decode with all sources that any group might need.
	tags, _, err := DecodeAll(Options{
		R:       f,
		Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES | XML,
	})
	c.Assert(err, qt.IsNil)

	golden := loadGolden(c, goldenPath)

	for _, group := range groups {
		goldenGroup, ok := golden[group]
		if !ok {
			continue
		}
		goldenMap, ok := goldenGroup.(map[string]any)
		c.Assert(ok, qt.IsTrue, qt.Commentf("golden %s is not a map", group))

		vmTags := goldenGroupTags(tags, group)
		c.Assert(vmTags != nil, qt.IsTrue, qt.Commentf("no videometa tags for group %s", group))

		compareAllGoldenTags(c, vmTags, goldenMap, group)
		assertExtraTagsNotInGolden(c, vmTags, goldenMap, group)
	}
}

// --- Exhaustive golden tests ---

// Validates: REQ-NF-04, REQ-TEST-03
func TestGoldenMinimalMP4(t *testing.T) {
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/minimal.mp4", "testdata/minimal.mp4.exiftool.json",
		[]string{"QuickTime", "Composite"})
}

// Validates: REQ-NF-04, REQ-QT-07, REQ-TEST-01
func TestGoldenWithGPS(t *testing.T) {
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/with_gps.mp4", "testdata/with_gps.mp4.exiftool.json",
		[]string{"QuickTime", "Composite"})
}

// Validates: REQ-NF-04, REQ-QT-04, REQ-XMP-04
func TestGoldenExifToolQuickTimeMOV(t *testing.T) {
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/exiftool_quicktime.mov", "testdata/exiftool_quicktime.mov.exiftool.json",
		[]string{"QuickTime", "MakerNotes", "XMP", "Composite"})
}

// Validates: REQ-NF-04, REQ-QT-04
func TestGoldenWithAudio(t *testing.T) {
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/with_audio.mp4", "testdata/with_audio.mp4.exiftool.json",
		[]string{"QuickTime", "Composite"})
}

// Validates: REQ-NF-04, REQ-BOX-05, REQ-TEST-05
func TestGoldenNonfaststart(t *testing.T) {
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/nonfaststart.mp4", "testdata/nonfaststart.mp4.exiftool.json",
		[]string{"QuickTime", "Composite"})
}

// Validates: REQ-NF-04, REQ-NF-06, REQ-TEST-04
func TestGoldenTruncated(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/truncated.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	// Truncated file — decode will error but may emit partial ftyp tags.
	tags, _, _ := DecodeAll(Options{R: f, Sources: QUICKTIME})

	golden := loadGolden(c, "testdata/truncated.mp4.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)
	qtTags := tags.QuickTime()

	// Verify whatever tags were emitted match exiftool.
	for name, goldenVal := range qtGolden {
		if tag, ok := qtTags[name]; ok {
			assertGoldenValue(c, "QuickTime", name, tag.Value, goldenVal)
		}
	}
}

// Validates: REQ-NF-04, REQ-TEST-09
func TestGoldenSonyA6700(t *testing.T) {
	if _, err := os.Stat("testdata/sony_a6700.mp4"); os.IsNotExist(err) {
		t.Skip("sony_a6700.mp4 not available")
	}
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/sony_a6700.mp4", "testdata/sony_a6700.mp4.exiftool.json",
		[]string{"QuickTime", "XML", "Composite"})
}

// Validates: REQ-NF-04, REQ-TEST-02
func TestGoldenAppleMOV(t *testing.T) {
	if _, err := os.Stat("testdata/apple.mov"); os.IsNotExist(err) {
		t.Skip("apple.mov not available")
	}
	c := qt.New(t)
	testGoldenExhaustive(c, "testdata/apple.mov", "testdata/apple.mov.exiftool.json",
		[]string{"QuickTime", "Composite"})
}

// --- Non-golden tests that use golden files ---

// Validates: REQ-API-13
func TestGoldenWithGPSLatLong(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	lat, lon, err := tags.GetLatLong()
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-34.0592) < 0.001, qt.IsTrue, qt.Commentf("lat=%f", lat))
	c.Assert(math.Abs(lon-(-118.446)) < 0.001, qt.IsTrue, qt.Commentf("lon=%f", lon))
}

// --- Helpers ---

func loadGolden(c *qt.C, path string) map[string]any {
	c.Helper()
	data, err := os.ReadFile(path)
	c.Assert(err, qt.IsNil)

	var results []map[string]any
	err = json.Unmarshal(data, &results)
	c.Assert(err, qt.IsNil)
	c.Assert(len(results) > 0, qt.IsTrue)
	return results[0]
}
