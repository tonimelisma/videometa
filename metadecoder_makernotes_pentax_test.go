package videometa

import (
	"encoding/binary"
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-04
func TestDecodePentaxTAGS(t *testing.T) {
	c := qt.New(t)

	// Build a synthetic Pentax TAGS binary blob matching the format from
	// exiftool's Pentax::MOV tag table (little-endian).
	data := make([]byte, 0xB1)
	le := binary.LittleEndian

	// Make: string[24] at offset 0x00.
	copy(data[0x00:], "PENTAX DIGITAL CAMERA\x00\x00\x00")

	// ExposureTime: int32u at 0x26, value conversion: 10 / raw.
	le.PutUint32(data[0x26:], 384) // 10/384 ≈ 0.02604

	// FNumber: rational64u at 0x2A (num/den).
	le.PutUint32(data[0x2A:], 40)
	le.PutUint32(data[0x2E:], 10) // 40/10 = 4.0

	// ExposureCompensation: rational64s at 0x32.
	le.PutUint32(data[0x32:], 0) // 0/10 = 0
	le.PutUint32(data[0x36:], 10)

	// WhiteBalance: int16u at 0x44.
	le.PutUint16(data[0x44:], 0) // Auto

	// FocalLength: rational64u at 0x48.
	le.PutUint32(data[0x48:], 189)
	le.PutUint32(data[0x4C:], 10) // 189/10 = 18.9

	// ISO: int16u at 0xAF.
	le.PutUint16(data[0xAF:], 50)

	// Decode through the public API by constructing a minimal decoder.
	var tags []TagInfo
	bd := &baseDecoder{
		streamReader: newStreamReader(nil),
		opts: Options{
			Sources: MAKERNOTES,
			HandleTag: func(ti TagInfo) error {
				tags = append(tags, ti)
				return nil
			},
		},
		result: &DecodeResult{},
	}
	dec := &videoDecoderMP4{baseDecoder: bd}
	dec.decodePentaxTAGS(data)

	c.Assert(len(tags), qt.Equals, 7)

	tagMap := make(map[string]any)
	for _, ti := range tags {
		c.Assert(ti.Source, qt.Equals, MAKERNOTES)
		c.Assert(ti.Namespace, qt.Equals, "MakerNotes")
		tagMap[ti.Tag] = ti.Value
	}

	c.Assert(tagMap["Make"], qt.Equals, "PENTAX DIGITAL CAMERA")
	c.Assert(math.Abs(tagMap["ExposureTime"].(float64)-0.026041666) < 0.0001, qt.IsTrue)
	c.Assert(tagMap["FNumber"], qt.Equals, 4.0)
	c.Assert(tagMap["ExposureCompensation"], qt.Equals, 0.0)
	c.Assert(tagMap["WhiteBalance"], qt.Equals, 0)
	c.Assert(tagMap["FocalLength"], qt.Equals, 18.9)
	c.Assert(tagMap["ISO"], qt.Equals, 50)
}

func TestDecodePentaxTAGSTooShort(t *testing.T) {
	c := qt.New(t)

	// Data too short for any field.
	bd := &baseDecoder{
		streamReader: newStreamReader(nil),
		opts: Options{
			Sources: MAKERNOTES,
			HandleTag: func(ti TagInfo) error {
				c.Fatal("should not emit tags from empty data")
				return nil
			},
		},
		result: &DecodeResult{},
	}
	dec := &videoDecoderMP4{baseDecoder: bd}
	dec.decodePentaxTAGS(make([]byte, 10))
}
