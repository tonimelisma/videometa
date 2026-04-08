package videometa

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-BOX-01, REQ-BOX-04, REQ-BOX-06, REQ-API-01
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

// Validates: REQ-NF-06, REQ-TEST-04
func TestDecodeIncompleteBoxMustError(t *testing.T) {
	c := qt.New(t)

	data := []byte{
		0, 0, 0, 20, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
	}

	_, err := Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: CONFIG,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
	})
	c.Assert(err, qt.IsNotNil, qt.Commentf("incomplete box payload must return error"))
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

	tags, _, err := decodeAllForTest(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	all := flattenAllTags(tags)
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

	s := QUICKTIME | VENDOR
	c.Assert(s.Has(QUICKTIME), qt.IsTrue)
	c.Assert(s.Has(VENDOR), qt.IsTrue)
	c.Assert(s.Has(CONFIG), qt.IsFalse)
	c.Assert(s.Has(COMPOSITE), qt.IsFalse)

	s = s.Remove(QUICKTIME)
	c.Assert(s.Has(QUICKTIME), qt.IsFalse)
	c.Assert(s.Has(VENDOR), qt.IsTrue)
}

// Validates: REQ-API-05
func TestSourceString(t *testing.T) {
	c := qt.New(t)

	c.Assert(QUICKTIME.String(), qt.Equals, "QUICKTIME")
	c.Assert((QUICKTIME | VENDOR | CONFIG).String(), qt.Equals, "QUICKTIME|VENDOR|CONFIG")
	c.Assert(Source(0).String(), qt.Equals, "0")
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

	tags, _, err := decodeAllForTest(Options{R: f, Sources: QUICKTIME})
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

// Validates: REQ-API-11, REQ-VENDOR-01
func TestTagsGetDateTimeVendorCreationDateValue(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{
		Source:    VENDOR,
		Namespace: "Sony/meta/nrtm",
		Tag:       "CreationDateValue",
		Value:     "2026-03-18T16:39:46-0700",
	})

	dt, err := tags.GetDateTime()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2026)
	c.Assert(dt.Month(), qt.Equals, time.March)
	c.Assert(dt.Day(), qt.Equals, 18)
	c.Assert(dt.Hour(), qt.Equals, 16)
}

// Validates: REQ-API-12
func TestTagsGetDateTimeUTC(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	dt, err := tags.GetDateTimeUTC()
	c.Assert(err, qt.IsNil)
	c.Assert(dt.Year(), qt.Equals, 2024)
	c.Assert(dt.Location(), qt.Equals, time.UTC)
}

// Validates: REQ-API-13
func TestTagsGetLatLongRealFile(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	lat, lon, err := tags.GetLatLong()
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-34.0592) < 0.001, qt.IsTrue, qt.Commentf("lat=%f", lat))
	c.Assert(math.Abs(lon-(-118.446)) < 0.001, qt.IsTrue, qt.Commentf("lon=%f", lon))
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

// Validates: REQ-API-07, REQ-API-09
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

// Validates: REQ-API-07
func TestLimitTagSize(t *testing.T) {
	c := qt.New(t)

	decodeWithLimit := func(limit uint32) map[string]TagInfo {
		f, err := os.Open("testdata/minimal.mp4")
		c.Assert(err, qt.IsNil)
		defer func() { _ = f.Close() }()

		tags := make(map[string]TagInfo)
		_, err = Decode(Options{
			R:            f,
			Sources:      QUICKTIME,
			LimitTagSize: limit,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
		})
		c.Assert(err, qt.IsNil)
		return tags
	}

	// Limit=5: MajorBrand "isom" (4 bytes, 4 > 5 false → passes).
	// CompressorName (long string → filtered). Non-string TimeScale (uint32 → not checked, passes).
	tags5 := decodeWithLimit(5)
	_, hasMajorBrand5 := tags5["MajorBrand"]
	c.Assert(hasMajorBrand5, qt.IsTrue, qt.Commentf("MajorBrand (4 bytes) should pass limit=5"))
	_, hasCompName5 := tags5["CompressorName"]
	c.Assert(hasCompName5, qt.IsFalse, qt.Commentf("CompressorName should be filtered at limit=5"))
	_, hasTimeScale5 := tags5["TimeScale"]
	c.Assert(hasTimeScale5, qt.IsTrue, qt.Commentf("non-string TimeScale should pass regardless of limit"))

	// Limit=4: MajorBrand "isom" (4 bytes, 4 > 4 false → passes). Proves > not >=.
	tags4 := decodeWithLimit(4)
	_, hasMajorBrand4 := tags4["MajorBrand"]
	c.Assert(hasMajorBrand4, qt.IsTrue, qt.Commentf("MajorBrand (4 bytes) should pass limit=4 (> not >=)"))

	// Limit=3: MajorBrand "isom" (4 bytes, 4 > 3 true → filtered).
	tags3 := decodeWithLimit(3)
	_, hasMajorBrand3 := tags3["MajorBrand"]
	c.Assert(hasMajorBrand3, qt.IsFalse, qt.Commentf("MajorBrand (4 bytes) should be filtered at limit=3"))
}

// Validates: REQ-API-02
func TestDecodeAllReturnsVideoConfig(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f, Sources: QUICKTIME | CONFIG})
	c.Assert(err, qt.IsNil)
	c.Assert(metadata.VideoConfig.Width, qt.Equals, 320)
	c.Assert(metadata.VideoConfig.Height, qt.Equals, 240)
	c.Assert(metadata.VideoConfig.Codec, qt.Equals, "avc1")
	for _, tag := range metadata.Tags.All() {
		c.Assert(tag.Source, qt.Not(qt.Equals), CONFIG)
	}
}

// Validates: REQ-API-19, REQ-API-22
func TestDecodeAllRealFilePreservesOrderedNamespaceAwareTags(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{
		R:       f,
		Sources: QUICKTIME | CONFIG,
	})
	c.Assert(err, qt.IsNil)

	all := metadata.Tags.All()
	c.Assert(len(all) >= 4, qt.IsTrue)
	c.Assert(all[0].Namespace, qt.Equals, "ftyp")
	c.Assert(all[0].Tag, qt.Equals, "MajorBrand")
	c.Assert(all[1].Namespace, qt.Equals, "ftyp")
	c.Assert(all[1].Tag, qt.Equals, "MinorVersion")

	timeScale, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/mvhd", "TimeScale")
	c.Assert(ok, qt.IsTrue)
	c.Assert(timeScale.Value, qt.Equals, uint32(1000))

	compressorID, ok := firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[1]/mdia/minf/stbl/stsd/video", "CompressorID")
	if !ok {
		compressorID, ok = firstTagInNamespace(metadata.Tags.QuickTime(), "moov/trak[1]/mdia/minf/stbl/stsd", "CompressorID")
	}
	c.Assert(ok, qt.IsTrue)
	c.Assert(compressorID.Value, qt.Equals, "avc1")
}

// Validates: REQ-API-05, REQ-API-20, REQ-API-21
func TestDecodeDefaultSourcesIncludeVendorAndConfigButNotComposite(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	var sawVendor bool
	var sawComposite bool
	result, err := Decode(Options{
		R: f,
		HandleTag: func(ti TagInfo) error {
			if ti.Source == VENDOR {
				sawVendor = true
			}
			if ti.Source == COMPOSITE {
				sawComposite = true
			}
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(sawVendor, qt.IsTrue)
	c.Assert(sawComposite, qt.IsFalse)
	c.Assert(result.VideoConfig.Codec, qt.Equals, "jpeg")
}

// Validates: REQ-API-05, REQ-API-20, REQ-API-21
func TestDecodeAllDefaultSourcesIncludeComposite(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{R: f})
	c.Assert(err, qt.IsNil)
	c.Assert(len(metadata.Tags.Composite().All()) > 0, qt.IsTrue)
	c.Assert(metadata.VideoConfig.Codec, qt.Equals, "avc1")
	for _, tag := range metadata.Tags.All() {
		c.Assert(tag.Source, qt.Not(qt.Equals), CONFIG)
	}
}

// Validates: REQ-API-09
func TestWarnfCallback(t *testing.T) {
	c := qt.New(t)

	data := buildMP4WithSonyNRTMIDAT([]byte(`<?xml version="1.0"?><NonRealTimeMeta`))

	var warnings []string
	_, _ = Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: VENDOR,
		HandleTag: func(ti TagInfo) error {
			return nil
		},
		Warnf: func(format string, args ...any) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		},
	})
	c.Assert(len(warnings) > 0, qt.IsTrue,
		qt.Commentf("Warnf should have been called for malformed Sony NRTM XML; got 0 warnings"))
	foundNRTMWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "decode sony nrtm") {
			foundNRTMWarning = true
		}
	}
	c.Assert(foundNRTMWarning, qt.IsTrue,
		qt.Commentf("expected warning about malformed Sony NRTM XML, got: %v", warnings))
}

// Validates: REQ-API-02
func TestTagsGetters(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: QUICKTIME, Tag: "Duration", Value: 5.0})
	tags.Add(TagInfo{Source: VENDOR, Tag: "DeviceModel", Namespace: "Sony/moov/meta/nrtm", Value: "A7"})
	tags.Add(TagInfo{Source: COMPOSITE, Tag: "ImageSize", Value: "1920 1080"})

	c.Assert(flattenSourceTags(tags.QuickTime())["Duration"].Value, qt.Equals, 5.0)
	c.Assert(flattenSourceTags(tags.Vendor())["DeviceModel"].Value, qt.Equals, "A7")
	c.Assert(flattenSourceTags(tags.Composite())["ImageSize"].Value, qt.Equals, "1920 1080")

	all := tags.All()
	c.Assert(len(all), qt.Equals, 3)
}

// Validates: REQ-API-16, REQ-API-19, REQ-API-22
func TestTagsPreserveNamespaceCollisions(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/mvhd", Tag: "HandlerType", Value: "mdir"})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/trak[1]/mdia/hdlr", Tag: "HandlerType", Value: "vide"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Sony/uuid/USMT", Tag: "TimeZone", Value: -420})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Sony/meta/nrtm", Tag: "TimeZone", Value: "PST"})

	c.Assert(len(tags.All()), qt.Equals, 4)
	c.Assert(tags.QuickTime().Namespaces(), qt.DeepEquals, []string{"moov/mvhd", "moov/trak[1]/mdia/hdlr"})
	c.Assert(tags.Vendor().Namespaces(), qt.DeepEquals, []string{"Sony/uuid/USMT", "Sony/meta/nrtm"})

	mvhdTag, ok := firstTagInNamespace(tags.QuickTime(), "moov/mvhd", "HandlerType")
	c.Assert(ok, qt.IsTrue)
	c.Assert(mvhdTag.Value, qt.Equals, "mdir")

	trackTag, ok := firstTagInNamespace(tags.QuickTime(), "moov/trak[1]/mdia/hdlr", "HandlerType")
	c.Assert(ok, qt.IsTrue)
	c.Assert(trackTag.Value, qt.Equals, "vide")

	timeZones := tags.Vendor().Find("TimeZone")
	c.Assert(len(timeZones), qt.Equals, 2)
	c.Assert(timeZones[0].Namespace, qt.Equals, "Sony/uuid/USMT")
	c.Assert(timeZones[1].Namespace, qt.Equals, "Sony/meta/nrtm")
}

// Validates: REQ-API-16, REQ-API-19
func TestTagsAllPreservesDecodeOrder(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "ftyp", Tag: "MajorBrand", Value: "isom"})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/mvhd", Tag: "TimeScale", Value: uint32(1000)})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "ISO", Value: 50})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/trak[1]/tkhd", Tag: "ImageWidth", Value: 320})

	all := tags.All()
	c.Assert(len(all), qt.Equals, 4)
	c.Assert(all[0].Tag, qt.Equals, "MajorBrand")
	c.Assert(all[1].Tag, qt.Equals, "TimeScale")
	c.Assert(all[2].Tag, qt.Equals, "ISO")
	c.Assert(all[3].Tag, qt.Equals, "ImageWidth")
}

// Validates: REQ-API-16, REQ-API-19
func TestSourceTagsFindPreservesDecodeOrderAcrossNamespaces(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/mvhd", Tag: "CreateDate", Value: "movie"})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/trak[1]/tkhd", Tag: "CreateDate", Value: "track"})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/trak[1]/mdia/mdhd", Tag: "CreateDate", Value: "media"})

	matches := tags.QuickTime().Find("CreateDate")
	c.Assert(matches, qt.HasLen, 3)
	c.Assert(matches[0].Namespace, qt.Equals, "moov/mvhd")
	c.Assert(matches[1].Namespace, qt.Equals, "moov/trak[1]/tkhd")
	c.Assert(matches[2].Namespace, qt.Equals, "moov/trak[1]/mdia/mdhd")
}

// Validates: REQ-API-16, REQ-API-19, REQ-API-22, REQ-VENDOR-03
func TestNamespaceTagsPreserveDuplicateTagsWithinNamespace(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Sony/uuid/USMT", Tag: "TrackProperty", Value: "16 0 0"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Sony/uuid/USMT", Tag: "TrackProperty", Value: "17 0 0"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Sony/uuid/USMT", Tag: "TimeZone", Value: -420})

	namespace := tags.Vendor().Namespace("Sony/uuid/USMT")
	c.Assert(namespace.All(), qt.HasLen, 3)

	trackProperties := namespace.Find("TrackProperty")
	c.Assert(trackProperties, qt.HasLen, 2)
	c.Assert(trackProperties[0].Value, qt.Equals, "16 0 0")
	c.Assert(trackProperties[1].Value, qt.Equals, "17 0 0")

	sourceMatches := tags.Vendor().FindInNamespace("Sony/uuid/USMT", "TrackProperty")
	c.Assert(sourceMatches, qt.DeepEquals, trackProperties)
}

// Validates: REQ-API-19, REQ-QT-01, REQ-QT-03
func TestQuickTimeNamespaceContractsRealFile(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{
		R:       f,
		Sources: QUICKTIME | VENDOR,
	})
	c.Assert(err, qt.IsNil)

	timeScale, ok := firstTagInNamespace(tags.QuickTime(), "moov/mvhd", "TimeScale")
	c.Assert(ok, qt.IsTrue)
	c.Assert(timeScale.Value, qt.Equals, uint32(1000))

	handlerType, ok := firstTagInNamespace(tags.QuickTime(), "moov/trak[1]/mdia/hdlr", "HandlerType")
	c.Assert(ok, qt.IsTrue)
	c.Assert(handlerType.Value, qt.Equals, "vide")

	compressorID, ok := firstTagInNamespace(tags.QuickTime(), "moov/trak[1]/mdia/minf/stbl/stsd", "CompressorID")
	c.Assert(ok, qt.IsTrue)
	c.Assert(compressorID.Value, qt.Equals, "avc1")

	if _, err := os.Stat("testdata/apple.mov"); os.IsNotExist(err) {
		t.Skip("testdata/apple.mov not present")
	}

	appleFile, err := os.Open("testdata/apple.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = appleFile.Close() }()

	appleTags, _, err := decodeAllForTest(Options{
		R:       appleFile,
		Sources: QUICKTIME,
	})
	c.Assert(err, qt.IsNil)

	makeTag, ok := firstTagInNamespace(appleTags.QuickTime(), "moov/meta/keys", "Make")
	c.Assert(ok, qt.IsTrue)
	c.Assert(makeTag.Value, qt.Equals, "Apple")
}

// Validates: REQ-API-19, REQ-VENDOR-01, REQ-VENDOR-02
func TestVendorNamespaceContractsRealFiles(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{
		R:       f,
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)

	iso, ok := firstTagInNamespace(tags.Vendor(), "Pentax/moov/udta/TAGS", "ISO")
	c.Assert(ok, qt.IsTrue)
	c.Assert(iso.Value, qt.Equals, 50)

	exposure, ok := firstTagInNamespace(tags.Vendor(), "Pentax/moov/udta/TAGS", "ExposureTime")
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(exposure.Value.(float64)-0.0260416666666667) < 0.0001, qt.IsTrue)

	if _, err := os.Stat("testdata/sony_a6700.mp4"); os.IsNotExist(err) {
		t.Skip("testdata/sony_a6700.mp4 not present")
	}

	sonyFile, err := os.Open("testdata/sony_a6700.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = sonyFile.Close() }()

	sonyTags, _, err := decodeAllForTest(Options{
		R:       sonyFile,
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)

	timeZone, ok := firstTagInNamespace(sonyTags.Vendor(), "Sony/uuid/USMT", "TimeZone")
	c.Assert(ok, qt.IsTrue)
	c.Assert(timeZone.Value, qt.Equals, -420)

	trackProperties := sonyTags.Vendor().Namespace("Sony/uuid/USMT").Find("TrackProperty")
	c.Assert(trackProperties, qt.HasLen, 3)
	c.Assert(trackProperties[0].Value, qt.Equals, "1 0 0")
	c.Assert(trackProperties[1].Value, qt.Equals, "1 0 0")
	c.Assert(trackProperties[2].Value, qt.Equals, "16 0 0")

	deviceModel, ok := firstTagInNamespace(sonyTags.Vendor(), "Sony/meta/nrtm", "DeviceModelName")
	c.Assert(ok, qt.IsTrue)
	c.Assert(deviceModel.Value, qt.Equals, "ILCE-6700")
}

// Validates: REQ-API-19
func TestCompositePrefersVendorMetadata(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/udta/meta/ilst/----", Tag: "FNumber", Value: 2.8})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/udta/meta/ilst/----", Tag: "ExposureTime", Value: 0.01})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/udta/meta/ilst/----", Tag: "FocalLength", Value: 35.0})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/udta/meta/ilst/----", Tag: "ISO", Value: 100})
	tags.Add(TagInfo{Source: QUICKTIME, Namespace: "moov/udta/meta/ilst/----", Tag: "LensModel", Value: "QuickTime Lens"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "FNumber", Value: 4.0})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "ExposureTime", Value: 0.025})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "FocalLength", Value: 18.9})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "ISO", Value: 50})
	tags.Add(TagInfo{Source: VENDOR, Namespace: "Pentax/moov/udta/TAGS", Tag: "LensModel", Value: "Vendor Lens"})

	computeComposite(&tags, VideoConfig{
		Width:    1920,
		Height:   1080,
		Rotation: 90,
	})

	aperture, ok := firstTagInNamespace(tags.Composite(), "Composite", "Aperture")
	c.Assert(ok, qt.IsTrue)
	c.Assert(aperture.Value, qt.Equals, 4.0)

	shutter, ok := firstTagInNamespace(tags.Composite(), "Composite", "ShutterSpeed")
	c.Assert(ok, qt.IsTrue)
	c.Assert(shutter.Value, qt.Equals, 0.025)

	focal, ok := firstTagInNamespace(tags.Composite(), "Composite", "FocalLength35efl")
	c.Assert(ok, qt.IsTrue)
	c.Assert(focal.Value, qt.Equals, 18.9)

	lensID, ok := firstTagInNamespace(tags.Composite(), "Composite", "LensID")
	c.Assert(ok, qt.IsTrue)
	c.Assert(lensID.Value, qt.Equals, "Vendor Lens")
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

// Validates: REQ-API-13, REQ-VENDOR-01
func TestTagsGetLatLongVendorSonyNamedGPS(t *testing.T) {
	c := qt.New(t)

	var tags Tags
	namespace := "Sony/meta/nrtm"
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemName", Value: "LatitudeRef"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemValue", Value: "N"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemName", Value: "Latitude"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemValue", Value: "29;19;10.922"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemName", Value: "LongitudeRef"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemValue", Value: "W"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemName", Value: "Longitude"})
	tags.Add(TagInfo{Source: VENDOR, Namespace: namespace, Tag: "AcquisitionRecordGroupItemValue", Value: "103;36;36.925"})

	lat, lon, err := tags.GetLatLong()
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-(29+19.0/60+10.922/3600)) < 0.000001, qt.IsTrue, qt.Commentf("lat=%f", lat))
	c.Assert(math.Abs(lon-(-(103+36.0/60+36.925/3600))) < 0.000001, qt.IsTrue, qt.Commentf("lon=%f", lon))
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
				c.Assert(ti.Namespace, qt.Equals, "moov/mvhd")
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

// Validates: REQ-API-14, REQ-CFG-01, REQ-CFG-02, REQ-CFG-03, REQ-CFG-04, REQ-QT-05
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

// Validates: REQ-BOX-02, REQ-TEST-10
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

	tags := make(map[string]TagInfo)
	_, err := Decode(Options{
		R:       readerSeekerFromBytes(data),
		Sources: QUICKTIME,
		HandleTag: func(ti TagInfo) error {
			tags[ti.Tag] = ti
			return nil
		},
	})
	// Valid ftyp + unknown box should parse without error.
	c.Assert(err, qt.IsNil)
	c.Assert(tags["MajorBrand"].Value, qt.Equals, "isom")
}

// Validates: REQ-QT-08
func TestQuickTimeCreationDateTimezone(t *testing.T) {
	c := qt.New(t)
	f, err := os.Open("testdata/with_gps.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{R: f, Sources: QUICKTIME})
	c.Assert(err, qt.IsNil)

	matches := tags.QuickTime().Find("CreationDate")
	c.Assert(len(matches) > 0, qt.IsTrue)
	cdStr, ok := matches[len(matches)-1].Value.(string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(cdStr, qt.Contains, "-07:00",
		qt.Commentf("CreationDate should preserve timezone, got %q", cdStr))
}

// Validates: REQ-API-16
func TestTagsSeparateBySource(t *testing.T) {
	c := qt.New(t)

	f, err := os.Open("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, err := decodeAllForTest(Options{
		R:       f,
		Sources: QUICKTIME | VENDOR,
	})
	c.Assert(err, qt.IsNil)

	// QuickTime-sourced tags.
	qtTags := flattenSourceTags(tags.QuickTime())
	c.Assert(len(qtTags) > 0, qt.IsTrue, qt.Commentf("no QuickTime tags"))
	_, hasTimeScale := qtTags["TimeScale"]
	c.Assert(hasTimeScale, qt.IsTrue, qt.Commentf("QuickTime should have TimeScale"))

	// Pentax TAGS are reclassified under Vendor.
	vendorTags := flattenSourceTags(tags.Vendor())
	_, hasISO := vendorTags["ISO"]
	c.Assert(hasISO, qt.IsTrue, qt.Commentf("Vendor should have ISO"))

	// Tags from different namespaces do not collide in the collected view.
	allTags := tags.All()
	c.Assert(len(allTags) > len(qtTags), qt.IsTrue,
		qt.Commentf("All() should contain more tags than flattened QuickTime alone"))
}

// Validates: REQ-API-18
func TestBestEffortPartial(t *testing.T) {
	c := qt.New(t)

	// Non-fast-start file (mdat before moov) with io.Reader (no seeking).
	// Should return partial data or graceful error, never panic.
	f, err := os.Open("testdata/nonfaststart.mp4")
	c.Assert(err, qt.IsNil)
	defer func() { _ = f.Close() }()

	tags, _, decodeErr := decodeAllForTest(Options{
		R:       readerOnly{f},
		Sources: QUICKTIME | CONFIG,
	})

	// nonfaststart.mp4 is ~5KB. Non-seekable reader can skip the entire mdat
	// via io.CopyN, so decode succeeds fully.
	c.Assert(decodeErr, qt.IsNil,
		qt.Commentf("5KB non-fast-start file should decode fully even without seeking"))
	all := flattenAllTags(tags)
	_, hasMajorBrand := all["MajorBrand"]
	c.Assert(hasMajorBrand, qt.IsTrue,
		qt.Commentf("ftyp tags should be emitted"))
	_, hasTimeScale := all["TimeScale"]
	c.Assert(hasTimeScale, qt.IsTrue,
		qt.Commentf("moov tags should be emitted after skipping small mdat"))
}

// Validates: ARCH-IO-05, REQ-API-18
func TestReaderOnlyLargeMdat(t *testing.T) {
	c := qt.New(t)

	// Synthetic non-fast-start MP4: ftyp + mdat (claims 100MB but only 8 bytes
	// of actual payload follow the header) + moov with mvhd.
	// With a non-seekable reader, the decoder must try to skip the mdat via
	// io.CopyN. Since the mdat claims to be huge but the stream ends early,
	// we expect an InvalidFormatError (EOF during skip).
	data := make([]byte, 0, 200)

	// ftyp box (20 bytes).
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm')

	// mdat box: header claims 100MB but only 8 bytes of padding follow.
	mdatSize := uint32(100 * 1024 * 1024)
	data = append(data, byte(mdatSize>>24), byte(mdatSize>>16), byte(mdatSize>>8), byte(mdatSize))
	data = append(data, 'm', 'd', 'a', 't')
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0) // 8 bytes of padding

	_, err := Decode(Options{
		R:         readerOnly{readerSeekerFromBytes(data)},
		Sources:   QUICKTIME | CONFIG,
		HandleTag: func(ti TagInfo) error { return nil },
	})
	// Stream ends before mdat can be fully skipped.
	c.Assert(err, qt.IsNotNil,
		qt.Commentf("large mdat with truncated stream should error"))
	c.Assert(IsInvalidFormat(err), qt.IsTrue,
		qt.Commentf("expected InvalidFormatError, got: %T: %v", err, err))
}

// Validates: REQ-BOX-03
func TestBoxExtendToEOF(t *testing.T) {
	c := qt.New(t)

	// Build synthetic MP4: ftyp (20 bytes) + moov with size=0 (extends to EOF).
	// moov contains a minimal mvhd.
	data := make([]byte, 0, 200)

	// ftyp box (20 bytes).
	data = append(data, 0, 0, 0, 20, 'f', 't', 'y', 'p')
	data = append(data, 'i', 's', 'o', 'm')
	data = append(data, 0, 0, 0, 0)
	data = append(data, 'i', 's', 'o', 'm')

	// moov box with size=0 (extends to EOF).
	data = append(data, 0, 0, 0, 0, 'm', 'o', 'o', 'v')

	// mvhd box inside moov (108 bytes for version 0).
	mvhdSize := uint32(108)
	data = append(data, byte(mvhdSize>>24), byte(mvhdSize>>16), byte(mvhdSize>>8), byte(mvhdSize))
	data = append(data, 'm', 'v', 'h', 'd')
	// version=0, flags=0
	data = append(data, 0, 0, 0, 0)
	// creation_time, modification_time (4 bytes each)
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0)
	// timescale = 1000
	data = append(data, 0, 0, 0x03, 0xE8)
	// duration = 5000 (5 seconds)
	data = append(data, 0, 0, 0x13, 0x88)
	// rate = 1.0 (0x00010000)
	data = append(data, 0, 1, 0, 0)
	// volume = 1.0 (0x0100)
	data = append(data, 1, 0)
	// reserved (10 bytes)
	data = append(data, make([]byte, 10)...)
	// matrix (36 bytes) — identity
	matrix := make([]byte, 36)
	// matrix[0] = 0x00010000 (1.0), matrix[16] = 0x00010000, matrix[32] = 0x40000000
	matrix[3] = 1
	matrix[4+12+3] = 1
	matrix[8+24] = 0x40
	data = append(data, matrix...)
	// pre_defined (24 bytes)
	data = append(data, make([]byte, 24)...)
	// next_track_ID
	data = append(data, 0, 0, 0, 1)

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
	// Should have parsed moov despite size=0 sentinel.
	c.Assert(tags["MajorBrand"].Value, qt.Equals, "isom")
	c.Assert(tags["TimeScale"].Value, qt.Equals, uint32(1000))
}

// Validates: REQ-NF-05
func TestSeedCorpusDecodesSuccessfully(t *testing.T) {
	// Ensure all committed valid test files decode without error.
	// Catches regressions where valid files start returning errors.
	files := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			c := qt.New(t)
			f, err := os.Open(path)
			c.Assert(err, qt.IsNil)
			defer func() { _ = f.Close() }()

			tagCount := 0
			_, err = Decode(Options{
				R:       f,
				Sources: QUICKTIME | VENDOR | CONFIG,
				HandleTag: func(ti TagInfo) error {
					tagCount++
					return nil
				},
			})
			c.Assert(err, qt.IsNil,
				qt.Commentf("valid file %s must decode without error", path))
			c.Assert(tagCount > 0, qt.IsTrue,
				qt.Commentf("valid file %s must produce at least one tag", path))
		})
	}
}

// Validates: REQ-NF-06, REQ-TEST-04
func TestKnownInvalidInputsMustError(t *testing.T) {
	// Counterpart to TestSeedCorpusDecodesSuccessfully — known-invalid inputs
	// must return InvalidFormatError, not succeed silently.
	inputs := []struct {
		name   string
		data   []byte
		reason string
	}{
		{
			name:   "truncated-ftyp-payload",
			data:   []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'},
			reason: "box payload truncated before declared size",
		},
		{
			name:   "truncated-box-header",
			data:   []byte{0, 0, 0, 8, 'f', 't'},
			reason: "box header truncated before fourcc",
		},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, err := Decode(Options{
				R:         readerSeekerFromBytes(tt.data),
				Sources:   QUICKTIME | CONFIG,
				HandleTag: func(ti TagInfo) error { return nil },
			})
			c.Assert(err, qt.IsNotNil,
				qt.Commentf("%s (%s) must return error", tt.name, tt.reason))
			c.Assert(IsInvalidFormat(err), qt.IsTrue,
				qt.Commentf("%s must return InvalidFormatError, got: %T: %v", tt.name, err, err))
		})
	}
}
