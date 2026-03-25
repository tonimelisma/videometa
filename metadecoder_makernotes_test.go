package videometa

import (
	"encoding/binary"
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-EXIF-07
func TestDecodeAppleMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("Apple", "", buildAppleMakerNotePayload("LIVE-ID-123"))
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.MakerNotes()["ContentIdentifier"].Value, qt.Equals, "LIVE-ID-123")
}

// Validates: REQ-EXIF-08
func TestDecodeCanonMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("Canon", "EOS R5", buildCanonMakerNotePayload())
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(tags.MakerNotes()["FocusMode"].Value, qt.Equals, 5)
	c.Assert(tags.MakerNotes()["CameraISO"].Value, qt.Equals, 100)
	c.Assert(tags.MakerNotes()["ImageStabilization"].Value, qt.Equals, 1)

	fNumber, ok := tags.MakerNotes()["FNumber"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(fNumber-4.0) < 1e-9, qt.IsTrue)

	exposureTime, ok := tags.MakerNotes()["ExposureTime"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(exposureTime-0.125) < 1e-9, qt.IsTrue)
}

// Validates: REQ-EXIF-09
func TestDecodeSonyMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("SONY", "ILCE-7M4", buildSonyMakerNotePayload())
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(tags.MakerNotes()["Quality"].Value, qt.Equals, uint32(5))
	c.Assert(tags.MakerNotes()["WhiteBalance"].Value, qt.Equals, uint32(0x20))

	flashExposureComp, ok := tags.MakerNotes()["FlashExposureComp"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(flashExposureComp-0.5) < 1e-9, qt.IsTrue)
}

func buildEXIFWithMakeModelAndMakerNotes(makeValue, modelValue string, makerNotesData []byte) []byte {
	type entry struct {
		tagID uint16
		typ   uint16
		data  []byte
	}

	entries := []entry{
		{tagID: 0x010F, typ: exifTypeASCII, data: append([]byte(makeValue), 0)},
	}
	if modelValue != "" {
		entries = append(entries, entry{tagID: 0x0110, typ: exifTypeASCII, data: append([]byte(modelValue), 0)})
	}
	entries = append(entries, entry{tagID: 0x927C, typ: exifTypeUndef, data: makerNotesData})

	buf := make([]byte, 1024)
	off := 0
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(buf[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(buf[off:], v)
		off += 4
	}

	buf[0], buf[1] = 'M', 'M'
	off = 2
	put16(0x002A)
	put32(8)

	put16(uint16(len(entries)))
	dataOffset := 8 + 2 + len(entries)*12 + 4
	currentDataOffset := dataOffset
	for _, entry := range entries {
		put16(entry.tagID)
		put16(entry.typ)
		put32(uint32(len(entry.data)))
		put32(uint32(currentDataOffset))
		currentDataOffset += len(entry.data)
	}
	put32(0)

	for _, entry := range entries {
		copy(buf[off:], entry.data)
		off += len(entry.data)
	}

	return buf[:off]
}

func buildAppleMakerNotePayload(contentIdentifier string) []byte {
	header := append([]byte("Apple iOS\x00"), 0x00, 0x00, 0x00, 0x00)
	payload := make([]byte, 256)
	copy(payload, header)

	off := len(header)
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	value := append([]byte(contentIdentifier), 0)
	put16(1)
	put16(0x0011)
	put16(exifTypeASCII)
	put32(uint32(len(value)))
	put32(uint32(len(header) + 2 + 12 + 4))
	put32(0)
	copy(payload[off:], value)
	off += len(value)

	return payload[:off]
}

func buildCanonMakerNotePayload() []byte {
	cameraSettings := make([]uint16, 34)
	cameraSettings[6] = 5   // FocusMode
	cameraSettings[15] = 17 // CameraISO => 100
	cameraSettings[24] = 10 // FocalUnits
	cameraSettings[33] = 1  // ImageStabilization

	shotInfo := make([]int16, 22)
	shotInfo[20] = 0x0080 // FNumber => 4.0
	shotInfo[21] = 0x0060 // ExposureTime => 1/8

	payload := make([]byte, 256)
	off := 0
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	put16(2)
	put16(0x0001)
	put16(exifTypeShort)
	put32(uint32(len(cameraSettings)))
	put32(30)

	put16(0x0004)
	put16(exifTypeSShort)
	put32(uint32(len(shotInfo)))
	put32(30 + uint32(len(cameraSettings))*2)

	put32(0)

	for _, v := range cameraSettings {
		put16(v)
	}
	for _, v := range shotInfo {
		put16(uint16(v))
	}

	return payload[:off]
}

func buildSonyMakerNotePayload() []byte {
	header := append([]byte("SONY DSC \x00"), 0x00, 0x00)
	payload := make([]byte, 256)
	copy(payload, header)

	off := len(header)
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	put16(3)
	put16(0x0102)
	put16(exifTypeLong)
	put32(1)
	put32(5)

	put16(0x0104)
	put16(exifTypeSRational)
	put32(1)
	put32(54)

	put16(0x0115)
	put16(exifTypeLong)
	put32(1)
	put32(0x20)

	put32(0)
	binary.BigEndian.PutUint32(payload[off:], 1)
	off += 4
	binary.BigEndian.PutUint32(payload[off:], 2)
	off += 4

	return payload[:off]
}
