package videometa

import (
	"fmt"
	"math"
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
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()

	_, err = Decode(Options{
		R:       f,
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
	})
	c.Assert(err, qt.IsNotNil, qt.Commentf("truncated file must return error"))
	c.Assert(IsInvalidFormat(err), qt.IsTrue, qt.Commentf("error: %v", err))
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
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	all := tags.All()
	c.Assert(all["TimeScale"].Value, qt.Equals, uint32(1000))
	c.Assert(all["ImageWidth"].Value, qt.Equals, 320)
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
	c.Assert(s.Has(COMPOSITE), qt.IsFalse)

	s = s.Remove(EXIF)
	c.Assert(s.Has(EXIF), qt.IsFalse)
	c.Assert(s.Has(XMP), qt.IsTrue)
}

// Validates: REQ-API-04
func TestDecodeAutoDetectFormat(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTime()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2024)
	c.Assert(dt.Month(), qt.Equals, time.Month(6))
	c.Assert(dt.Day(), qt.Equals, 15)
	c.Assert(dt.Hour(), qt.Equals, 10)
	c.Assert(dt.Minute(), qt.Equals, 30)
	c.Assert(dt.Second(), qt.Equals, 0)
}

// Validates: REQ-API-12
func TestTagsGetDateTimeUTC(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTimeUTC()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2024)
	c.Assert(dt.Location(), qt.Equals, time.UTC)
}

// Validates: REQ-API-10
func TestDecodeTimeout(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	_, err = Decode(Options{
		R:         &slowReader{rs: f, delay: 100 * time.Millisecond},
		Sources:   CONFIG,
		Timeout:   50 * time.Millisecond,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNotNil, qt.Commentf("decode should have timed out"))
	c.Assert(err.Error(), qt.Contains, "timed out")
}

// Validates: REQ-API-10
func TestDecodeTimeoutNotExceeded(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()

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

// Validates: ARCH-IO-05, REQ-API-03
func TestDecodeWithIOReaderFallback(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	result, err := Decode(Options{
		R:         readerOnly{f},
		Sources:   CONFIG | QUICKTIME,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
}

// Validates: REQ-API-17
func TestDecodeNoMetadataFile(t *testing.T) {
	c := qt.New(t)

	data := make([]byte, 0, 20)
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 0)
	data = append(data, 'i', 's', 'o', 'm')

	tags := make(map[string]TagInfo)
	_, err := Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: QUICKTIME | CONFIG,
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags["MajorBrand"].Value, qt.Equals, "isom")
	c.Assert(tags["MinorVersion"].Value, qt.IsNotNil)
	_, hasCB := tags["CompatibleBrands"]
	c.Assert(hasCB, qt.IsTrue)
	c.Assert(len(tags), qt.Equals, 3)
}

// Validates: REQ-API-08, REQ-API-09
func TestLimitNumTags(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	count := 0
	_, err = Decode(Options{
		R:            f,
		Sources:      QUICKTIME,
		LimitNumTags: 5,
		HandleTag: func(ti TagInfo) error {
			count++
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 5)
}

// Validates: REQ-API-08
func TestLimitTagSize(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags := make(map[string]TagInfo)
	_, err = Decode(Options{
		R:            f,
		Sources:      QUICKTIME,
		LimitTagSize: 5,
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	_, hasCompName := tags["CompressorName"]
	c.Assert(hasCompName, qt.IsFalse, qt.Commentf("long CompressorName should be skipped"))
	_, hasMajorBrand := tags["MajorBrand"]
	c.Assert(hasMajorBrand, qt.IsTrue, qt.Commentf("short MajorBrand should be present"))
}

// Validates: REQ-API-02
func TestDecodeAllReturnsVideoConfig(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	_, result, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "avc1")
}

// Validates: REQ-API-09
func TestWarnfCallback(t *testing.T) {
	c := qt.New(t)

	data := buildMP4WithInvalidEXIF()

	var warnings []string
	_, _ = Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: QUICKTIME | EXIF,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
		Warnf: func(format string, args ...any) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		},
	})
	c.Assert(len(warnings) > 0, qt.IsTrue,
		qt.Commentf("Warnf should have been called for invalid EXIF data; got 0 warnings"))
}

// Validates: REQ-API-02
func TestTagsGetters(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: EXIF, Tag: "Make", Value: "Canon"})
	tags.Add(TagInfo{Source: XMP, Tag: "Creator", Value: "Test"})
	tags.Add(TagInfo{Source: IPTC, Tag: "City", Value: "NYC"})
	tags.Add(TagInfo{Source: QUICKTIME, Tag: "Duration", Value: 5.0})
	tags.Add(TagInfo{Source: CONFIG, Tag: "Width", Value: 1920})
	tags.Add(TagInfo{Source: MAKERNOTES, Tag: "ISO", Value: 100})
	tags.Add(TagInfo{Source: XML, Tag: "DeviceModel", Value: "A7"})
	tags.Add(TagInfo{Source: COMPOSITE, Tag: "ImageSize", Value: "1920 1080"})

	c.Assert(tags.EXIF()["Make"].Value, qt.Equals, "Canon")
	c.Assert(tags.XMP()["Creator"].Value, qt.Equals, "Test")
	c.Assert(tags.IPTC()["City"].Value, qt.Equals, "NYC")
	c.Assert(tags.QuickTime()["Duration"].Value, qt.Equals, 5.0)
	c.Assert(tags.Config()["Width"].Value, qt.Equals, 1920)
	c.Assert(tags.MakerNotes()["ISO"].Value, qt.Equals, 100)
	c.Assert(tags.XML()["DeviceModel"].Value, qt.Equals, "A7")
	c.Assert(tags.Composite()["ImageSize"].Value, qt.Equals, "1920 1080")

	all := tags.All()
	c.Assert(len(all), qt.Equals, 8)
}

// Validates: REQ-API-13
func TestTagsGetLatLongQuickTime(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	// GPSCoordinates is now in exiftool space-separated format after conversion.
	tags.Add(TagInfo{Source: QUICKTIME, Tag: "GPSCoordinates", Value: "34.0592 -118.446 42.938"})

	lat, lon, err := tags.GetLatLong()
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-34.0592) < 0.001, qt.IsTrue, qt.Commentf("lat=%f", lat))
	c.Assert(math.Abs(lon-(-118.446)) < 0.001, qt.IsTrue, qt.Commentf("lon=%f", lon))
}

// Validates: REQ-API-13
func TestTagsGetLatLongNoGPS(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	_, _, err := tags.GetLatLong()
	c.Assert(err, qt.IsNotNil)
}

// Validates: REQ-API-06
func TestHandleTagFieldsPopulated(t *testing.T) {
	c := qt.New(t)
	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	var found bool
	_, err = Decode(Options{
		R:       f,
		Sources: QUICKTIME,
		HandleTag: func(ti TagInfo) error {
			if ti.Tag == "TimeScale" {
				c.Assert(ti.Source, qt.Equals, QUICKTIME)
				c.Assert(ti.Namespace, qt.Equals, "QuickTime")
				c.Assert(ti.Tag, qt.Equals, "TimeScale")
				c.Assert(ti.Value, qt.Equals, uint32(1000))
				found = true
			}
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(found, qt.IsTrue)
}

// Validates: REQ-API-14, REQ-CFG-01, REQ-CFG-02, REQ-CFG-03, REQ-CFG-04
func TestVideoConfig(t *testing.T) {
	c := qt.New(t)
	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	result, err := Decode(Options{
		R:         f,
		Sources:   CONFIG,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.VideoConfig.Width, qt.Equals, 320)
	c.Assert(result.VideoConfig.Height, qt.Equals, 240)
	c.Assert(result.VideoConfig.Rotation, qt.Equals, 0)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "avc1")
	c.Assert(result.VideoConfig.Duration > 0, qt.IsTrue,
		qt.Commentf("Duration should be > 0, got %v", result.VideoConfig.Duration))
}

// Validates: REQ-BOX-02
func TestBox64BitExtendedSize(t *testing.T) {
	c := qt.New(t)

	data := make([]byte, 0, 40)
	// Box header: size=1 (signals 64-bit), type=ftyp
	data = append(data, 0, 0, 0, 1, 'f', 't', 'y', 'p')
	// 64-bit size: 28 bytes total
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 28)
	// ftyp body: brand=isom, version=0, compat=isom
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 0)
	data = append(data, 'i', 's', 'o', 'm')

	tags := make(map[string]TagInfo)
	_, err := Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: QUICKTIME,
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags["MajorBrand"].Value, qt.Equals, "isom")
}

// Validates: REQ-BOX-07
func TestBoxSkipUnknown(t *testing.T) {
	c := qt.New(t)

	data := make([]byte, 0, 36)
	// ftyp box (20 bytes)
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm')
	// Unknown box "zzzz" (16 bytes)
	data = append(data, 0, 0, 0, 16, 'z', 'z', 'z', 'z')
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0)

	_, err := Decode(Options{
		R:         readerSeekerFromBytes(data),
		Sources:   QUICKTIME,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	// Should not panic on unknown box.
	if err != nil {
		c.Assert(IsInvalidFormat(err), qt.IsTrue)
	}
}

// Validates: REQ-QT-08
func TestQuickTimeCreationDateTimezone(t *testing.T) {
	c := qt.New(t)
	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := DecodeAll(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	cd, ok := tags.QuickTime()["CreationDate"]
	c.Assert(ok, qt.IsTrue)
	cdStr := toString(cd.Value)
	c.Assert(cdStr, qt.Contains, "-07:00",
		qt.Commentf("CreationDate should preserve timezone, got %q", cdStr))
}

// Validates: REQ-EXIF-04
func TestEXIFFieldTableSize(t *testing.T) {
	c := qt.New(t)
	c.Assert(len(exifFields) >= 100, qt.IsTrue,
		qt.Commentf("exifFields has %d entries, expected >= 100", len(exifFields)))
	c.Assert(len(exifFieldsGPS) >= 30, qt.IsTrue,
		qt.Commentf("exifFieldsGPS has %d entries, expected >= 30", len(exifFieldsGPS)))
}

// Validates: REQ-EXIF-07
func TestAppleMakerNotes(t *testing.T) {
	t.Skip("Apple MakerNotes decoding not yet implemented")
}

// Validates: REQ-EXIF-08
func TestCanonMakerNotes(t *testing.T) {
	t.Skip("Canon MakerNotes decoding not yet implemented")
}

// Validates: REQ-EXIF-09
func TestSonyEXIFMakerNotes(t *testing.T) {
	t.Skip("Sony EXIF MakerNotes decoding not yet implemented")
}
