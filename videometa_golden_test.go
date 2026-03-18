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

// Validates: REQ-NF-04, REQ-QT-04
func TestGoldenExifToolQuickTimeMOV(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	golden := loadGolden(c, "testdata/exiftool_quicktime.mov.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	qtTags := tags.QuickTime()
	c.Assert(len(qtTags) > 0, qt.IsTrue, qt.Commentf("expected QuickTime tags from exiftool test MOV"))

	// Basic structure.
	c.Assert(qtTags["TimeScale"].Value, qt.Equals, uint32(600))

	// Audio tags — this MOV has a raw audio track.
	compareGoldenTag(c, qtTags, qtGolden, "AudioFormat")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioChannels")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioBitsPerSample")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioSampleRate")
	compareGoldenNumTag(c, qtTags, qtGolden, "Balance")

	// VendorID from visual sample entry.
	compareGoldenTag(c, qtTags, qtGolden, "VendorID")

	// Track aperture mode dimensions from tapt box.
	compareGoldenTag(c, qtTags, qtGolden, "CleanApertureDimensions")
	compareGoldenTag(c, qtTags, qtGolden, "ProductionApertureDimensions")
	compareGoldenTag(c, qtTags, qtGolden, "EncodedPixelsDimensions")

	// HandlerClass from mdia hdlr.
	compareGoldenTag(c, qtTags, qtGolden, "HandlerClass")

	// ImageWidth/Height should be from video track, not overwritten by audio track.
	c.Assert(qtTags["ImageWidth"].Value, qt.Equals, int(qtGolden["ImageWidth"].(float64)))
	c.Assert(qtTags["ImageHeight"].Value, qt.Equals, int(qtGolden["ImageHeight"].(float64)))

	// TrackDuration uses movie timescale (600), not hardcoded 1000.
	trackDur, ok := qtTags["TrackDuration"]
	c.Assert(ok, qt.IsTrue)
	goldenDur := qtGolden["TrackDuration"].(float64)
	c.Assert(math.Abs(trackDur.Value.(float64)-goldenDur) < 0.01, qt.IsTrue,
		qt.Commentf("TrackDuration: got %v, want %v", trackDur.Value, goldenDur))
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

	// CompatibleBrands — verify our []string matches golden's []any.
	cb, ok := qtTags["CompatibleBrands"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing CompatibleBrands tag"))
	cbSlice, ok := cb.Value.([]string)
	c.Assert(ok, qt.IsTrue, qt.Commentf("CompatibleBrands should be []string"))
	goldenBrands := qtGolden["CompatibleBrands"].([]any)
	c.Assert(len(cbSlice), qt.Equals, len(goldenBrands))
	for i, gb := range goldenBrands {
		c.Assert(cbSlice[i], qt.Equals, gb.(string), qt.Commentf("CompatibleBrands[%d]", i))
	}
}

// Validates: REQ-NF-04, REQ-QT-04
func TestGoldenWithAudioMP4(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_audio.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, result, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	golden := loadGolden(c, "testdata/with_audio.mp4.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	qtTags := tags.QuickTime()

	// Video tags should be present and correct.
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "avc1")
	c.Assert(qtTags["ImageWidth"].Value, qt.Equals, 320)
	c.Assert(qtTags["ImageHeight"].Value, qt.Equals, 240)
	compareGoldenTag(c, qtTags, qtGolden, "CompressorID")

	// Audio tags from the audio track.
	compareGoldenTag(c, qtTags, qtGolden, "AudioFormat")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioChannels")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioBitsPerSample")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioSampleRate")
	compareGoldenNumTag(c, qtTags, qtGolden, "Balance")

	// VideoFrameRate should be present (from stts of video track).
	c.Assert(qtTags["VideoFrameRate"].Value, qt.Equals, float64(qtGolden["VideoFrameRate"].(float64)))
}

// Validates: REQ-NF-04
func TestGoldenSonyA6700(t *testing.T) {
	if _, err := os.Stat("testdata/sony_a6700.mp4"); os.IsNotExist(err) {
		t.Skip("sony_a6700.mp4 not available (large file, not committed)")
	}
	c := qt.New(t)

	f, err := os.Open("testdata/sony_a6700.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, result, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	golden := loadGolden(c, "testdata/sony_a6700.mp4.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	qtTags := tags.QuickTime()

	// HEVC codec + 4K dimensions.
	c.Assert(result.VideoConfig.Codec, qt.Equals, "hvc1")
	c.Assert(result.VideoConfig.Width, qt.Equals, 3840)
	c.Assert(result.VideoConfig.Height, qt.Equals, 2160)
	compareGoldenTag(c, qtTags, qtGolden, "MajorBrand")
	compareGoldenTag(c, qtTags, qtGolden, "CompressorID")
	c.Assert(qtTags["ImageWidth"].Value, qt.Equals, int(qtGolden["ImageWidth"].(float64)))
	c.Assert(qtTags["ImageHeight"].Value, qt.Equals, int(qtGolden["ImageHeight"].(float64)))

	// LPCM audio track.
	compareGoldenTag(c, qtTags, qtGolden, "AudioFormat")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioChannels")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioBitsPerSample")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioSampleRate")
	compareGoldenNumTag(c, qtTags, qtGolden, "Balance")

	// TrackDuration should use movie timescale (60000).
	trackDur, ok := qtTags["TrackDuration"]
	c.Assert(ok, qt.IsTrue)
	goldenDur := qtGolden["TrackDuration"].(float64)
	c.Assert(math.Abs(trackDur.Value.(float64)-goldenDur) < 0.01, qt.IsTrue,
		qt.Commentf("TrackDuration: got %v, want %v", trackDur.Value, goldenDur))
}

// Validates: REQ-NF-04
func TestGoldenAppleMOV(t *testing.T) {
	if _, err := os.Stat("testdata/apple.mov"); os.IsNotExist(err) {
		t.Skip("apple.mov not available (large file, not committed)")
	}
	c := qt.New(t)

	f, err := os.Open("testdata/apple.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, result, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)

	golden := loadGolden(c, "testdata/apple.mov.exiftool.json")
	qtGolden := golden["QuickTime"].(map[string]any)

	qtTags := tags.QuickTime()

	// HEVC codec + 4K dimensions.
	c.Assert(result.VideoConfig.Codec, qt.Equals, "hvc1")
	c.Assert(result.VideoConfig.Width, qt.Equals, 3840)
	c.Assert(result.VideoConfig.Height, qt.Equals, 2160)
	compareGoldenTag(c, qtTags, qtGolden, "MajorBrand")
	compareGoldenTag(c, qtTags, qtGolden, "CompressorID")
	c.Assert(qtTags["ImageWidth"].Value, qt.Equals, int(qtGolden["ImageWidth"].(float64)))
	c.Assert(qtTags["ImageHeight"].Value, qt.Equals, int(qtGolden["ImageHeight"].(float64)))

	// Audio track.
	compareGoldenTag(c, qtTags, qtGolden, "AudioFormat")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioChannels")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioBitsPerSample")
	compareGoldenNumTag(c, qtTags, qtGolden, "AudioSampleRate")
	compareGoldenNumTag(c, qtTags, qtGolden, "Balance")

	// Apple freeform metadata.
	compareGoldenTag(c, qtTags, qtGolden, "Make")
	compareGoldenTag(c, qtTags, qtGolden, "Model")
	compareGoldenTag(c, qtTags, qtGolden, "Software")
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

// compareGoldenNumTag checks that a numeric tag's value matches the golden float64.
func compareGoldenNumTag(c *qt.C, tags map[string]TagInfo, golden map[string]any, name string) {
	c.Helper()
	tag, ok := tags[name]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing tag %q", name))
	goldenVal, ok := golden[name]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing golden tag %q", name))
	tagFloat, ok := toFloat64(tag.Value)
	c.Assert(ok, qt.IsTrue, qt.Commentf("tag %q: cannot convert %T to float64", name, tag.Value))
	goldenFloat, ok := toFloat64(goldenVal)
	c.Assert(ok, qt.IsTrue, qt.Commentf("golden %q: cannot convert %T to float64", name, goldenVal))
	c.Assert(math.Abs(tagFloat-goldenFloat) < 0.001, qt.IsTrue,
		qt.Commentf("tag %q: got %v, want %v", name, tagFloat, goldenFloat))
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
