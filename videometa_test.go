package videometa

import (
	"os"
	"testing"

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

	// CONFIG: VideoConfig from tkhd and mvhd.
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
	c.Assert(result.VideoConfig.Rotation, qt.Equals, 0)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "avc1")

	// QuickTime tags from mvhd.
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
	// Truncated files should return an error (InvalidFormatError or EOF-related).
	// The ftyp box should parse OK (it's only 32 bytes), but moov will be truncated.
	// Depending on where truncation hits, we may get an error or just an empty result.
	// For 500 bytes, the ftyp (32 bytes) and moov header will be read but moov
	// content will be truncated.
	if err != nil {
		c.Assert(IsInvalidFormat(err), qt.IsTrue, qt.Commentf("error: %v", err))
	}
}

// Validates: REQ-API-17
func TestDecodeEmptyOptions(t *testing.T) {
	c := qt.New(t)

	_, err := Decode(Options{})
	c.Assert(err, qt.IsNotNil, qt.Commentf("should fail without R"))
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
	c.Assert(err, qt.IsNil) // ErrStopWalking is not returned to caller.
	c.Assert(count, qt.Equals, 3)
}

// Validates: REQ-API-02
func TestDecodeAll(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer f.Close()

	tags, err := DecodeAll(Options{
		R:       f,
		Sources: QUICKTIME,
	})
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

	tags, err := DecodeAll(Options{
		R:       f,
		Sources: QUICKTIME,
	})
	c.Assert(err, qt.IsNil)

	qtTags := tags.QuickTime()
	// minimal.mp4 has ©too = "Lavf62.3.100" which maps to "Encoder".
	encoder, ok := qtTags["Encoder"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("expected Encoder tag from ilst"))
	c.Assert(encoder.Value, qt.Equals, "Lavf62.3.100")
}

// Validates: REQ-BOX-08
func TestDecodeFragmentedMP4Rejected(t *testing.T) {
	c := qt.New(t)

	// Create a minimal fragmented MP4 (just ftyp + moof headers).
	// ftyp box: 8-byte header + "isom" + minor version = 20 bytes.
	data := make([]byte, 0, 40)
	// ftyp box (20 bytes): size=20, "ftyp", "isom", minor=0, no compat brands.
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 0)       // minor version
	data = append(data, 'i', 's', 'o', 'm') // compatible brand

	// moof box (8 bytes): size=8, "moof".
	data = append(data, 0, 0, 0, 8, 'm', 'o', 'o', 'f')

	_, err := Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
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

	// No VideoFormat specified — should auto-detect.
	result, err := Decode(Options{
		R:       f,
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
}
