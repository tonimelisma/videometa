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
