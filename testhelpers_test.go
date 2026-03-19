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
