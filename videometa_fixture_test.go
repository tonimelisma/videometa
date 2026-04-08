package videometa

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-TEST-06
func TestCommittedGoogleFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertFixtureAvailable(c, committedGoogleFixture)
}

// Validates: REQ-TEST-07
func TestBootstrappedGoProFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertBootstrappedFixtureAvailable(c, bootstrappedGoProFixture)
}

// Validates: REQ-TEST-08
func TestBootstrappedDJIInspire3FixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertBootstrappedFixtureAvailable(c, bootstrappedDJIInspireFixture)
}

// Validates: REQ-TEST-09
func TestBootstrappedDJIRonin4DFixtureAvailable(t *testing.T) {
	c := qt.New(t)
	assertBootstrappedFixtureAvailable(c, bootstrappedDJIRoninFixture)
}

// Validates: REQ-QT-04, REQ-CFG-04
func TestGoogleMP4ColorProfileTags(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open(committedGoogleFixture)
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
	c := qt.New(t)

	f := openBootstrappedFixture(t, bootstrappedGoProFixture)
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

func assertFixtureAvailable(c *qt.C, mediaPath string) {
	c.Helper()

	f, err := os.Open(mediaPath)
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME | VENDOR | CONFIG})
	c.Assert(err, qt.IsNil)
	c.Assert(len(metadata.Tags.All()) > 0, qt.IsTrue)
}

func assertBootstrappedFixtureAvailable(c *qt.C, mediaPath string) {
	c.Helper()

	f := openBootstrappedFixture(c, mediaPath)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME | VENDOR | CONFIG})
	c.Assert(err, qt.IsNil)
	c.Assert(len(metadata.Tags.All()) > 0, qt.IsTrue)
}
