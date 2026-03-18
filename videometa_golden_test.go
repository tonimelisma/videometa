package videometa

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-04, REQ-QT-07
func TestGoldenWithGPSQuickTimeTags(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	// Load golden file for comparison.
	golden := loadGolden(c, "testdata/with_gps.mp4.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	// Compare key QuickTime tags against exiftool output.
	qtTags := tags.QuickTime()

	// TimeScale
	c.Assert(qtTags["TimeScale"].Value, qt.Equals, uint32(1000))
	c.Assert(qtGolden["TimeScale"], qt.Equals, float64(1000)) // JSON numbers are float64

	// ImageWidth/Height
	c.Assert(qtTags["ImageWidth"].Value, qt.Equals, 320)
	c.Assert(qtTags["ImageHeight"].Value, qt.Equals, 240)

	// GPS from freeform atoms (com.apple.quicktime.location.ISO6709)
	gps, ok := qtTags["GPSCoordinates"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("expected GPSCoordinates from freeform atom"))
	c.Assert(gps.Value, qt.IsNotNil)

	// Make and Model from freeform atoms
	make_, ok := qtTags["Make"]
	c.Assert(ok, qt.IsTrue)
	c.Assert(make_.Value, qt.Equals, "TestCamera")

	model, ok := qtTags["Model"]
	c.Assert(ok, qt.IsTrue)
	c.Assert(model.Value, qt.Equals, "TestModel")

	// CreationDate with timezone
	cd, ok := qtTags["CreationDate"]
	c.Assert(ok, qt.IsTrue)
	c.Assert(cd.Value, qt.IsNotNil)
}

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

// Validates: REQ-API-11
func TestGoldenWithGPSGetDateTime(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTime()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2024)
	c.Assert(dt.Month(), qt.Equals, time.Month(6))
	c.Assert(dt.Day(), qt.Equals, 15)
}

// Validates: REQ-NF-04
func TestGoldenExifToolQuickTimeMOV(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	// This file is from exiftool test suite — rich QuickTime metadata.
	qtTags := tags.QuickTime()

	// Should have parsed basic QuickTime structure.
	c.Assert(len(qtTags) > 0, qt.IsTrue, qt.Commentf("expected QuickTime tags from exiftool test MOV"))

	// Check codec.
	all := tags.All()
	ts, ok := all["TimeScale"]
	c.Assert(ok, qt.IsTrue)
	c.Assert(ts.Value, qt.Equals, uint32(600))
}

// Validates: REQ-NF-04
func TestGoldenMinimalMP4AllQuickTimeTags(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)
	qtTags := tags.QuickTime()

	golden := loadGolden(c, "testdata/minimal.mp4.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	// Compare all golden QuickTime tags that we emit.
	compareGoldenTag(c, qtTags, qtGolden, "MajorBrand")
	compareGoldenTag(c, qtTags, qtGolden, "MinorVersion")
	compareGoldenTag(c, qtTags, qtGolden, "MatrixStructure")
	compareGoldenTag(c, qtTags, qtGolden, "HandlerDescription")
	compareGoldenTag(c, qtTags, qtGolden, "GraphicsMode")
	compareGoldenTag(c, qtTags, qtGolden, "OpColor")
	compareGoldenTag(c, qtTags, qtGolden, "CompressorName")
	compareGoldenTag(c, qtTags, qtGolden, "PixelAspectRatio")

	// Numeric comparisons.
	c.Assert(qtTags["VideoFrameRate"].Value, qt.Equals, float64(qtGolden["VideoFrameRate"].(float64)))
	c.Assert(qtTags["SourceImageWidth"].Value, qt.Equals, int(qtGolden["SourceImageWidth"].(float64)))
	c.Assert(qtTags["SourceImageHeight"].Value, qt.Equals, int(qtGolden["SourceImageHeight"].(float64)))
	c.Assert(qtTags["BitDepth"].Value, qt.Equals, int(qtGolden["BitDepth"].(float64)))
	c.Assert(qtTags["XResolution"].Value, qt.Equals, int(qtGolden["XResolution"].(float64)))
	c.Assert(qtTags["YResolution"].Value, qt.Equals, int(qtGolden["YResolution"].(float64)))
}

// compareGoldenTag checks that a string-valued tag matches the golden file.
func compareGoldenTag(c *qt.C, tags map[string]TagInfo, golden map[string]any, name string) {
	c.Helper()
	tag, ok := tags[name]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing tag %q", name))
	goldenVal, ok := golden[name]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing golden tag %q", name))
	c.Assert(toString(tag.Value), qt.Equals, toString(goldenVal),
		qt.Commentf("tag %q: got %v, want %v", name, tag.Value, goldenVal))
}

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
