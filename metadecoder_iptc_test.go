package videometa

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
)

// buildIPTCData creates a minimal IPTC byte stream.
func buildIPTCData(records ...iptcRecord) []byte {
	var buf bytes.Buffer
	for _, r := range records {
		buf.WriteByte(iptcMarker) // 0x1C
		buf.WriteByte(r.record)
		buf.WriteByte(r.dataset)
		size := len(r.data)
		buf.WriteByte(byte(size >> 8))
		buf.WriteByte(byte(size & 0xFF))
		buf.Write(r.data)
	}
	return buf.Bytes()
}

type iptcRecord struct {
	record  uint8
	dataset uint8
	data    []byte
}

// Validates: REQ-IPTC-01
func TestDecodeIPTCBasic(t *testing.T) {
	c := qt.New(t)

	data := buildIPTCData(
		iptcRecord{record: 2, dataset: 5, data: []byte("Test Title")},
		iptcRecord{record: 2, dataset: 120, data: []byte("Test Caption")},
		iptcRecord{record: 2, dataset: 80, data: []byte("John Doe")},
	)

	tags := decodeIPTCFromBytes(c, data)

	c.Assert(tags["ObjectName"].Value, qt.Equals, "Test Title")
	c.Assert(tags["Caption-Abstract"].Value, qt.Equals, "Test Caption")
	c.Assert(tags["By-line"].Value, qt.Equals, "John Doe")
	c.Assert(tags["ObjectName"].Source, qt.Equals, IPTC)
}

// Validates: REQ-IPTC-03
func TestDecodeIPTCRepeatable(t *testing.T) {
	c := qt.New(t)

	data := buildIPTCData(
		iptcRecord{record: 2, dataset: 25, data: []byte("landscape")},
		iptcRecord{record: 2, dataset: 25, data: []byte("sunset")},
		iptcRecord{record: 2, dataset: 25, data: []byte("nature")},
	)

	tags := decodeIPTCFromBytes(c, data)

	keywords, ok := tags["Keywords"].Value.([]string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(keywords, qt.DeepEquals, []string{"landscape", "sunset", "nature"})
}

// Validates: REQ-IPTC-02
func TestDecodeIPTCSingleKeyword(t *testing.T) {
	c := qt.New(t)

	data := buildIPTCData(
		iptcRecord{record: 2, dataset: 25, data: []byte("solo")},
	)

	tags := decodeIPTCFromBytes(c, data)
	c.Assert(tags["Keywords"].Value, qt.Equals, "solo")
}

// Validates: REQ-IPTC-02
func TestDecodeIPTCCharsets(t *testing.T) {
	c := qt.New(t)

	c.Run("UTF-8 charset", func(c *qt.C) {
		data := buildIPTCData(
			iptcRecord{record: 1, dataset: 90, data: []byte{0x1B, 0x25, 0x47}}, // ESC % G = UTF-8
			iptcRecord{record: 2, dataset: 5, data: []byte("Ünîcödé Tïtlé")},
		)
		tags := decodeIPTCFromBytes(c, data)
		c.Assert(tags["ObjectName"].Value, qt.Equals, "Ünîcödé Tïtlé")
	})

	c.Run("ISO-8859-1 default", func(c *qt.C) {
		// No charset record → default UTF-8, but data has high bytes that
		// happen to be valid UTF-8 since Go strings are UTF-8 by default.
		// To test ISO-8859-1 decoding, use an explicit non-UTF-8 indicator.
		data := buildIPTCData(
			iptcRecord{record: 1, dataset: 90, data: []byte{0x1B, 0x2D, 0x41}}, // ESC - A = ISO-8859-1
			iptcRecord{record: 2, dataset: 5, data: []byte{0xE9, 0xE8, 0xEA}},  // éèê in ISO-8859-1
		)
		tags := decodeIPTCFromBytes(c, data)
		c.Assert(tags["ObjectName"].Value, qt.Equals, "éèê")
	})

	c.Run("explicit ISO-8859-1 full string", func(c *qt.C) {
		data := buildIPTCData(
			iptcRecord{record: 1, dataset: 90, data: []byte{0x1B, 0x2D, 0x41}},                                // ESC - A
			iptcRecord{record: 2, dataset: 105, data: []byte{0x48, 0xE9, 0x61, 0x64, 0x6C, 0x69, 0x6E, 0xE9}}, // "Héadliné"
		)
		tags := decodeIPTCFromBytes(c, data)
		c.Assert(tags["Headline"].Value, qt.Equals, "Héadliné")
	})
}

// Validates: REQ-IPTC-04
func TestDecodeIPTCViaApplicationNotes(t *testing.T) {
	c := qt.New(t)

	// Build IPTC data.
	iptcData := buildIPTCData(
		iptcRecord{record: 2, dataset: 5, data: []byte("IPTC Title")},
		iptcRecord{record: 2, dataset: 25, data: []byte("embedded")},
	)

	// Wrap in EXIF ApplicationNotes tag (0x83BB).
	exifData := buildEXIFWithIPTC(iptcData)

	// Decode with both EXIF and IPTC sources enabled.
	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(bytes.NewReader(nil)),
		opts: Options{
			Sources: EXIF | IPTC,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
			Warnf: func(f string, a ...any) { c.Logf("WARN: "+f, a...) },
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}
	d.decodeEXIF(bytes.NewReader(exifData))

	// IPTC tags should be present (routed from EXIF ApplicationNotes).
	c.Assert(tags["ObjectName"].Value, qt.Equals, "IPTC Title")
	c.Assert(tags["ObjectName"].Source, qt.Equals, IPTC)
	c.Assert(tags["Keywords"].Value, qt.Equals, "embedded")
}

func decodeIPTCFromBytes(c *qt.C, data []byte) map[string]TagInfo {
	c.Helper()
	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(bytes.NewReader(nil)),
		opts: Options{
			Sources: IPTC,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}
	d.decodeIPTC(bytes.NewReader(data))
	return tags
}
