package videometa

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
)

// readerSeekerFromBytes creates an io.ReadSeeker from a byte slice.
func readerSeekerFromBytes(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
}

// buildMP4WithInvalidEXIF builds a minimal valid MP4 with ftyp + uuid (EXIF)
// box containing garbage data. This triggers the EXIF decoder's Warnf callback
// for invalid byte order marker.
func buildMP4WithInvalidEXIF() []byte {
	var buf bytes.Buffer

	// ftyp box (20 bytes).
	_ = binary.Write(&buf, binary.BigEndian, uint32(20)) // size
	buf.WriteString("ftyp")
	buf.WriteString("isom")                             // major brand
	_ = binary.Write(&buf, binary.BigEndian, uint32(0)) // minor version
	buf.WriteString("isom")                             // compat brand

	// uuid box with EXIF UUID + invalid EXIF data.
	// Format: size(4) + "uuid"(4) + uuid(16) + header_offset(4) + garbage(8)
	exifBody := make([]byte, 12)
	binary.BigEndian.PutUint32(exifBody[0:4], 0) // header offset = 0
	copy(exifBody[4:], []byte("BADEXIF!"))       // invalid EXIF data (not "MM" or "II")

	boxSize := uint32(8 + 16 + len(exifBody))
	_ = binary.Write(&buf, binary.BigEndian, boxSize)
	buf.WriteString("uuid")
	buf.Write(exifUUID[:])
	buf.Write(exifBody)

	return buf.Bytes()
}

// buildEXIFWithMakerNotes creates an EXIF/TIFF structure with a MakerNotes tag (0x927C).
func buildEXIFWithMakerNotes(makerNotesData []byte) []byte {
	buf := make([]byte, 256)
	off := 0
	bo := binary.BigEndian

	put16 := func(v uint16) { bo.PutUint16(buf[off:], v); off += 2 }
	put32 := func(v uint32) { bo.PutUint32(buf[off:], v); off += 4 }

	// TIFF header: MM + 0x002A + IFD0 offset.
	buf[0], buf[1] = 'M', 'M'
	off = 2
	put16(0x002A)
	put32(8) // IFD0 at offset 8

	// IFD0: 1 tag.
	put16(1)

	// Tag: MakerNotes (0x927C), UNDEFINED, count=len(data), offset→data.
	put16(0x927C)
	put16(exifTypeUndef)
	put32(uint32(len(makerNotesData)))
	dataOff := 8 + 2 + 12 + 4 // IFD start + count(2) + 1 tag(12) + nextIFD(4)
	put32(uint32(dataOff))

	// Next IFD = 0.
	put32(0)

	// MakerNotes data.
	copy(buf[off:], makerNotesData)
	off += len(makerNotesData)

	return buf[:off]
}

// buildEXIFWithIPTC creates an EXIF/TIFF structure with ApplicationNotes tag (0x83BB)
// containing IPTC data.
func buildEXIFWithIPTC(iptcData []byte) []byte {
	buf := make([]byte, 512)
	off := 0
	bo := binary.BigEndian

	put16 := func(v uint16) { bo.PutUint16(buf[off:], v); off += 2 }
	put32 := func(v uint32) { bo.PutUint32(buf[off:], v); off += 4 }

	// TIFF header.
	buf[0], buf[1] = 'M', 'M'
	off = 2
	put16(0x002A)
	put32(8) // IFD0 at offset 8

	// IFD0: 1 tag.
	put16(1)

	// Tag: ApplicationNotes (0x83BB), UNDEFINED, count=len(iptcData), offset→data.
	put16(0x83BB)
	put16(exifTypeUndef)
	put32(uint32(len(iptcData)))
	dataOff := 8 + 2 + 12 + 4
	put32(uint32(dataOff))

	// Next IFD = 0.
	put32(0)

	// IPTC data.
	copy(buf[off:], iptcData)
	off += len(iptcData)

	return buf[:off]
}

// slowReader wraps an io.ReadSeeker and adds delay to each Read call.
type slowReader struct {
	rs    io.ReadSeeker
	delay time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	time.Sleep(s.delay)
	return s.rs.Read(p)
}

func (s *slowReader) Seek(offset int64, whence int) (int64, error) {
	return s.rs.Seek(offset, whence)
}
