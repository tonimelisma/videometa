package videometa

import (
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-BOX-01, REQ-BOX-06, REQ-API-01
func TestDecodeMinimalMP4(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags := make(map[string]TagInfo)
	result, err := Decode(Options{
		R:       f,
		Sources: QUICKTIME | CONFIG,
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	c.Assert(err, qt.IsNil)

	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
	c.Assert(result.VideoConfig.Rotation, qt.Equals, 0)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "avc1")

	c.Assert(tags["TimeScale"].Value, qt.Equals, uint32(1000))
	c.Assert(tags["ImageWidth"].Value, qt.Equals, 320)
	c.Assert(tags["ImageHeight"].Value, qt.Equals, 240)
	c.Assert(tags["CompressorID"].Value, qt.Equals, "avc1")
}

// Validates: REQ-BOX-05
func TestDecodeNonFastStartMP4(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/nonfaststart.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	result, err := Decode(Options{
		R:       f,
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
}

// Validates: REQ-NF-06
func TestDecodeTruncatedMP4(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/truncated.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	_, err = Decode(Options{
		R:       f,
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
	})
	if err != nil {
		c.Assert(IsInvalidFormat(err), qt.IsTrue, qt.Commentf("error: %v", err))
	}
}

// Validates: REQ-API-17
func TestDecodeEmptyOptions(t *testing.T) {
	c := qt.New(t)
	_, err := Decode(Options{})
	c.Assert(err, qt.IsNotNil)
}

// Validates: REQ-API-15
func TestDecodeStopWalking(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	count := 0
	_, err = Decode(Options{
		R:       f,
		Sources: QUICKTIME,
		HandleTag: func(ti TagInfo) error {
			count++
			if count >= 3 {
				return ErrStopWalking
			}
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)
}

// Validates: REQ-API-02
func TestDecodeAll(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	all := tags.All()
	c.Assert(len(all) > 0, qt.IsTrue)
	c.Assert(all["TimeScale"].Value, qt.Equals, uint32(1000))
}

// Validates: REQ-QT-01
func TestDecodeIlstEncoder(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	qtTags := tags.QuickTime()
	encoder, ok := qtTags["Encoder"]
	c.Assert(ok, qt.IsTrue)
	c.Assert(encoder.Value, qt.Equals, "Lavf62.3.100")
}

// Validates: REQ-BOX-08
func TestDecodeFragmentedMP4Rejected(t *testing.T) {
	c := qt.New(t)

	data := make([]byte, 0, 40)
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 0)
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 8, 'm', 'o', 'o', 'f')

	_, err := Decode(Options{
		R:         readerSeekerFromBytes(data),
		Sources:   CONFIG,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(IsInvalidFormat(err), qt.IsTrue)
}

// Validates: REQ-API-05
func TestSourceBitmask(t *testing.T) {
	c := qt.New(t)

	s := EXIF | XMP
	c.Assert(s.Has(EXIF), qt.IsTrue)
	c.Assert(s.Has(XMP), qt.IsTrue)
	c.Assert(s.Has(IPTC), qt.IsFalse)

	s = s.Remove(EXIF)
	c.Assert(s.Has(EXIF), qt.IsFalse)
	c.Assert(s.Has(XMP), qt.IsTrue)
}

// Validates: REQ-API-04
func TestDecodeAutoDetectFormat(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	result, err := Decode(Options{
		R:         f,
		Sources:   CONFIG,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
}

// Validates: REQ-API-11
func TestTagsGetDateTime(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTime()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2024)
	c.Assert(dt.Month(), qt.Equals, time.Month(6))
	c.Assert(dt.Day(), qt.Equals, 15)
}

// Validates: REQ-API-12
func TestTagsGetDateTimeUTC(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTimeUTC()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Location(), qt.Equals, time.UTC)
}

// Validates: REQ-API-10
func TestDecodeTimeout(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	_, err = Decode(Options{
		R:         f,
		Sources:   CONFIG,
		Timeout:   5 * time.Second,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNil)
}

// Validates: REQ-API-07
func TestShouldHandleTag(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags := make(map[string]TagInfo)
	_, err = Decode(Options{
		R:       f,
		Sources: QUICKTIME,
		ShouldHandleTag: func(ti TagInfo) bool {
			return ti.Tag == "TimeScale"
		},
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(tags), qt.Equals, 1)
	c.Assert(tags["TimeScale"].Value, qt.Equals, uint32(1000))
}
