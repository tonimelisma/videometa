package videometa

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// buildMinimalEXIF creates a minimal valid EXIF/TIFF structure in a byte buffer.
func buildMinimalEXIF(byteOrder binary.ByteOrder) []byte {
	var buf bytes.Buffer
	w := func(v any) { _ = binary.Write(&buf, byteOrder, v) }

	// Byte order marker.
	if byteOrder == binary.LittleEndian {
		buf.Write([]byte("II"))
	} else {
		buf.Write([]byte("MM"))
	}

	// Magic number 0x002A.
	w(uint16(0x002A))

	// IFD0 offset (immediately after header = offset 8).
	w(uint32(8))

	// IFD0: 2 tags.
	w(uint16(2))

	// Tag 1: Make (0x010F), ASCII, count=6, value="Apple\0"
	w(uint16(0x010F))
	w(uint16(exifTypeASCII))
	w(uint32(6))
	// Value fits in 4 bytes? No, 6 bytes. So this is an offset.
	// Offset to string data (after all IFD entries + next IFD pointer).
	// IFD: 2 + (2*12) + 4 = 30 bytes from start of IFD (offset 8).
	// So string data starts at offset 8 + 30 = 38.
	w(uint32(38))

	// Tag 2: Model (0x0110), ASCII, count=11, value="iPhone 15\0"
	w(uint16(0x0110))
	w(uint16(exifTypeASCII))
	w(uint32(10))
	w(uint32(44)) // offset to string data = 38 + 6

	// Next IFD offset (0 = no more IFDs).
	w(uint32(0))

	// String data for Make.
	buf.Write([]byte("Apple\x00"))

	// String data for Model.
	buf.Write([]byte("iPhone 15\x00"))

	return buf.Bytes()
}

// Validates: REQ-EXIF-01, REQ-EXIF-02
func TestDecodeEXIFBigEndian(t *testing.T) {
	c := qt.New(t)

	exifData := buildMinimalEXIF(binary.BigEndian)
	tags := decodeEXIFFromBytes(c, exifData)

	c.Assert(tags["Make"].Value, qt.Equals, "Apple")
	c.Assert(tags["Model"].Value, qt.Equals, "iPhone 15")
	c.Assert(tags["Make"].Source, qt.Equals, EXIF)
	c.Assert(tags["Make"].Namespace, qt.Equals, "IFD0")
}

// Validates: REQ-EXIF-02
func TestDecodeEXIFLittleEndian(t *testing.T) {
	c := qt.New(t)

	exifData := buildMinimalEXIF(binary.LittleEndian)
	tags := decodeEXIFFromBytes(c, exifData)

	c.Assert(tags["Make"].Value, qt.Equals, "Apple")
	c.Assert(tags["Model"].Value, qt.Equals, "iPhone 15")
}

// Validates: REQ-EXIF-05
func TestDecodeEXIFGPSCoordinates(t *testing.T) {
	c := qt.New(t)

	// Build EXIF with GPS sub-IFD.
	exifData := buildEXIFWithGPS(binary.BigEndian)
	tags := decodeEXIFFromBytes(c, exifData)

	// Should have converted GPS DMS to decimal degrees.
	lat, ok := tags["GPSLatitude"]
	c.Assert(ok, qt.IsTrue, qt.Commentf("missing GPSLatitude"))
	latVal, ok := lat.Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(latVal-34.05920) < 0.001, qt.IsTrue,
		qt.Commentf("lat: got %f, want ~34.0592", latVal))
}

// buildEXIFWithGPS creates an EXIF structure with a GPS sub-IFD.
// Uses explicit offset tracking to avoid calculation errors.
func buildEXIFWithGPS(byteOrder binary.ByteOrder) []byte {
	buf := make([]byte, 256)
	off := 0

	put16 := func(v uint16) {
		byteOrder.PutUint16(buf[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		byteOrder.PutUint32(buf[off:], v)
		off += 4
	}
	putStr := func(s string) {
		copy(buf[off:], s)
		off += len(s)
	}

	// TIFF header.
	if byteOrder == binary.LittleEndian {
		putStr("II")
	} else {
		putStr("MM")
	}
	put16(0x002A)
	put32(8) // IFD0 offset

	// IFD0 at offset 8: 1 tag.
	ifd0Off := off // should be 8
	_ = ifd0Off
	put16(1) // tag count

	// GPSInfo pointer (0x8825) → GPS IFD at offset 26.
	put16(0x8825)
	put16(exifTypeLong)
	put32(1)
	gpsIFDOff := 8 + 2 + 12 + 4 // 26
	put32(uint32(gpsIFDOff))

	// Next IFD = 0.
	put32(0)

	// GPS IFD at offset 26: 5 tags.
	put16(5)

	// GPSLatitudeRef: "N\0" inline (2 bytes + 2 pad).
	put16(0x0001)
	put16(exifTypeASCII)
	put32(2)
	buf[off] = 'N'
	buf[off+1] = 0
	buf[off+2] = 0
	buf[off+3] = 0
	off += 4

	// GPSLatitude: 3 rationals → offset to data area.
	put16(0x0002)
	put16(exifTypeRational)
	put32(3)
	dataAreaOff := gpsIFDOff + 2 + 5*12 + 4 // 26 + 66 = 92
	put32(uint32(dataAreaOff))

	// GPSLongitudeRef: "W\0" inline.
	put16(0x0003)
	put16(exifTypeASCII)
	put32(2)
	buf[off] = 'W'
	buf[off+1] = 0
	buf[off+2] = 0
	buf[off+3] = 0
	off += 4

	// GPSLongitude: 3 rationals → offset.
	put16(0x0004)
	put16(exifTypeRational)
	put32(3)
	put32(uint32(dataAreaOff + 24))

	// GPSAltitude: 1 rational → offset.
	put16(0x0006)
	put16(exifTypeRational)
	put32(1)
	put32(uint32(dataAreaOff + 48))

	// Next IFD = 0.
	put32(0)

	// Data area should start at dataAreaOff (92).
	// Pad if needed.
	for off < dataAreaOff {
		buf[off] = 0
		off++
	}

	// Lat: 34/1, 3/1, 33/1
	put32(34)
	put32(1)
	put32(3)
	put32(1)
	put32(33)
	put32(1)

	// Lon: 118/1, 26/1, 45/1
	put32(118)
	put32(1)
	put32(26)
	put32(1)
	put32(45)
	put32(1)

	// Alt: 42/1
	put32(42)
	put32(1)

	return buf[:off]
}

// decodeEXIFFromBytes is a test helper that decodes EXIF from a byte slice.
func decodeEXIFFromBytes(c *qt.C, data []byte) map[string]TagInfo {
	c.Helper()
	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(bytes.NewReader(nil)),
		opts: Options{
			Sources: EXIF,
			HandleTag: func(ti TagInfo) error {
				c.Logf("EXIF tag: %s ns=%s val=%v (%T)", ti.Tag, ti.Namespace, ti.Value, ti.Value)
				tags[ti.Tag] = ti
				return nil
			},
			Warnf: func(f string, a ...any) { c.Logf("WARN: "+f, a...) },
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}
	d.decodeEXIF(bytes.NewReader(data))
	return tags
}
