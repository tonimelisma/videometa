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
