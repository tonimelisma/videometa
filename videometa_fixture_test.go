package videometa

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-TEST-06
func TestLocalGoogleFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertLocalFixtureAvailable(c, "testdata/google.mp4")
}

// Validates: REQ-TEST-07
func TestLocalGoProFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertLocalFixtureAvailable(c, "testdata/gopro_action.mp4")
}

// Validates: REQ-TEST-08
func TestLocalDJIInspire3FixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertLocalFixtureAvailable(c, "testdata/dji_inspire3_car_4k120_rec709.mov")
}

// Validates: REQ-TEST-09
func TestLocalDJIRonin4DFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertLocalFixtureAvailable(c, "testdata/dji_ronin4d_4k_prores4444_25fps.mov")
}

// Validates: REQ-QT-04
func TestGoogleMP4ColorProfileTags(t *testing.T) {
	if _, err := os.Stat("testdata/google.mp4"); os.IsNotExist(err) {
		t.Skip("google.mp4 not available")
	}

	c := qt.New(t)

	f, err := os.Open("testdata/google.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	colorProfile, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[3]/mdia/minf/stbl/stsd/colr", "ColorProfiles")
	c.Assert(ok, qt.IsTrue)
	c.Assert(colorProfile.Value, qt.Equals, "nclx")

	colorPrimaries, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[3]/mdia/minf/stbl/stsd/colr", "ColorPrimaries")
	c.Assert(ok, qt.IsTrue)
	c.Assert(colorPrimaries.Value, qt.Equals, 9)

	transferCharacteristics, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[3]/mdia/minf/stbl/stsd/colr", "TransferCharacteristics")
	c.Assert(ok, qt.IsTrue)
	c.Assert(transferCharacteristics.Value, qt.Equals, 18)

	matrixCoefficients, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[3]/mdia/minf/stbl/stsd/colr", "MatrixCoefficients")
	c.Assert(ok, qt.IsTrue)
	c.Assert(matrixCoefficients.Value, qt.Equals, 9)

	fullRangeFlag, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[3]/mdia/minf/stbl/stsd/colr", "VideoFullRangeFlag")
	c.Assert(ok, qt.IsTrue)
	c.Assert(fullRangeFlag.Value, qt.Equals, 0)
}

// Validates: REQ-QT-04, REQ-TEST-07
func TestGoProMP4RepeatedVendorTags(t *testing.T) {
	if _, err := os.Stat("testdata/gopro_action.mp4"); os.IsNotExist(err) {
		t.Skip("gopro_action.mp4 not available")
	}

	c := qt.New(t)

	f, err := os.Open("testdata/gopro_action.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME | VENDOR})
	c.Assert(err, qt.IsNil)

	deviceNames := metadata.Tags.Vendor().FindInNamespace("GoPro/moov/udta/GPMF", "DeviceName")
	c.Assert(deviceNames, qt.HasLen, 3)
	c.Assert(deviceNames[0].Value, qt.Equals, "Global Settings")
	c.Assert(deviceNames[1].Value, qt.Equals, "Large FOV")
	c.Assert(deviceNames[2].Value, qt.Equals, "Highlights")

	videoFrameRates := metadata.Tags.Vendor().FindInNamespace("GoPro/moov/udta/GPMF", "VideoFrameRate")
	c.Assert(videoFrameRates, qt.HasLen, 1)
	c.Assert(videoFrameRates[0].Value, qt.Equals, "30000 1001")
}

func assertLocalFixtureAvailable(c *qt.C, mediaPath string) {
	c.Helper()

	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		c.Skip(mediaPath + " not available")
	}

	f, err := os.Open(mediaPath)
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME | VENDOR | CONFIG})
	c.Assert(err, qt.IsNil)
	c.Assert(len(metadata.Tags.All()) > 0, qt.IsTrue)
}
