package videometa

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-04, REQ-TEST-09
func TestSonyA6700NRTMExposesGPSItems(t *testing.T) {
	if _, err := os.Stat("testdata/sony_a6700.mp4"); os.IsNotExist(err) {
		t.Skip("sony_a6700.mp4 not available")
	}

	c := qt.New(t)

	f, err := os.Open("testdata/sony_a6700.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{
		R:       f,
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)

	names := metadata.Tags.Vendor().FindInNamespace("Sony/meta/nrtm", "AcquisitionRecordGroupItemName")
	values := metadata.Tags.Vendor().FindInNamespace("Sony/meta/nrtm", "AcquisitionRecordGroupItemValue")

	var gotNames []any
	var gotValues []any
	for _, tag := range names {
		gotNames = append(gotNames, tag.Value)
	}
	for _, tag := range values {
		gotValues = append(gotValues, tag.Value)
	}

	c.Assert(gotNames, qt.Contains, any("LatitudeRef"))
	c.Assert(gotNames, qt.Contains, any("Latitude"))
	c.Assert(gotNames, qt.Contains, any("LongitudeRef"))
	c.Assert(gotNames, qt.Contains, any("Longitude"))

	c.Assert(gotValues, qt.Contains, any("N"))
	c.Assert(gotValues, qt.Contains, any("29;19;10.922"))
	c.Assert(gotValues, qt.Contains, any("W"))
	c.Assert(gotValues, qt.Contains, any("103;36;36.925"))
}
