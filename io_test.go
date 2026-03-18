package videometa

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-01
func TestStreamReaderRead1(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x42, 0xFF, 0x00}
	sr := newStreamReader(bytes.NewReader(data))

	c.Assert(sr.read1(), qt.Equals, uint8(0x42))
	c.Assert(sr.read1(), qt.Equals, uint8(0xFF))
	c.Assert(sr.read1(), qt.Equals, uint8(0x00))
}

// Validates: REQ-NF-01
func TestStreamReaderRead2BigEndian(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x00, 0x2A, 0xFF, 0xFF}
	sr := newStreamReader(bytes.NewReader(data))
	sr.byteOrder = binary.BigEndian

	c.Assert(sr.read2(), qt.Equals, uint16(42))
	c.Assert(sr.read2(), qt.Equals, uint16(0xFFFF))
}

// Validates: REQ-NF-01
func TestStreamReaderRead2LittleEndian(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x2A, 0x00}
	sr := newStreamReader(bytes.NewReader(data))
	sr.byteOrder = binary.LittleEndian

	c.Assert(sr.read2(), qt.Equals, uint16(42))
}

// Validates: REQ-NF-01
func TestStreamReaderRead4(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(123456))
	sr := newStreamReader(bytes.NewReader(buf.Bytes()))

	c.Assert(sr.read4(), qt.Equals, uint32(123456))
}

// Validates: REQ-NF-01
func TestStreamReaderRead8(t *testing.T) {
	c := qt.New(t)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint64(0xDEADBEEFCAFEBABE))
	sr := newStreamReader(bytes.NewReader(buf.Bytes()))

	c.Assert(sr.read8(), qt.Equals, uint64(0xDEADBEEFCAFEBABE))
}

// Validates: REQ-NF-01
func TestStreamReaderReadBytes(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	sr := newStreamReader(bytes.NewReader(data))

	got := sr.readBytes(3)
	c.Assert(got, qt.DeepEquals, []byte{0x01, 0x02, 0x03})

	got = sr.readBytes(2)
	c.Assert(got, qt.DeepEquals, []byte{0x04, 0x05})
}

// Validates: REQ-NF-01
func TestStreamReaderReadFourCC(t *testing.T) {
	c := qt.New(t)
	data := []byte("ftyp")
	sr := newStreamReader(bytes.NewReader(data))

	fcc := sr.readFourCC()
	c.Assert(fcc.String(), qt.Equals, "ftyp")
}

// Validates: REQ-NF-01
func TestStreamReaderPos(t *testing.T) {
	c := qt.New(t)
	data := make([]byte, 100)
	sr := newStreamReader(bytes.NewReader(data))

	c.Assert(sr.pos(), qt.Equals, int64(0))
	sr.read4()
	c.Assert(sr.pos(), qt.Equals, int64(4))
	sr.read2()
	c.Assert(sr.pos(), qt.Equals, int64(6))
}

// Validates: REQ-NF-01
func TestStreamReaderSeek(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x42}
	sr := newStreamReader(bytes.NewReader(data))

	sr.seek(4)
	c.Assert(sr.read1(), qt.Equals, uint8(0x42))
}

// Validates: REQ-NF-01
func TestStreamReaderSkipSeekable(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x42}
	sr := newStreamReader(bytes.NewReader(data))

	sr.skip(4)
	c.Assert(sr.read1(), qt.Equals, uint8(0x42))
}

// Validates: ARCH-IO-05
func TestStreamReaderSkipNonSeekable(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x42}
	// Wrap in a struct that only implements io.Reader (not io.ReadSeeker).
	sr := newStreamReader(readerOnly{bytes.NewReader(data)})
	c.Assert(sr.canSeek, qt.IsFalse)

	sr.skip(4)
	c.Assert(sr.read1(), qt.Equals, uint8(0x42))
}

// Validates: REQ-NF-01
func TestStreamReaderPreservePos(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x00, 0x01, 0x02, 0x03, 0x42}
	sr := newStreamReader(bytes.NewReader(data))

	sr.read2()
	c.Assert(sr.pos(), qt.Equals, int64(2))

	sr.preservePos(func() {
		sr.seek(4)
		c.Assert(sr.read1(), qt.Equals, uint8(0x42))
	})

	c.Assert(sr.pos(), qt.Equals, int64(2))
}

// Validates: REQ-NF-01
func TestStreamReaderBufferedReader(t *testing.T) {
	c := qt.New(t)
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}
	sr := newStreamReader(bytes.NewReader(data))

	// Read first 4 bytes into a buffered reader.
	rc := sr.bufferedReader(4)
	defer func() { _ = rc.Close() }()

	buf := make([]byte, 4)
	_, err := io.ReadFull(rc, buf)
	c.Assert(err, qt.IsNil)
	c.Assert(buf, qt.DeepEquals, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	// Verify the buffered reader is seekable.
	_, err = rc.Seek(0, io.SeekStart)
	c.Assert(err, qt.IsNil)
	_, err = io.ReadFull(rc, buf)
	c.Assert(err, qt.IsNil)
	c.Assert(buf, qt.DeepEquals, []byte{0xDE, 0xAD, 0xBE, 0xEF})

	// Verify main reader advanced past the buffered bytes.
	c.Assert(sr.pos(), qt.Equals, int64(4))
	c.Assert(sr.read1(), qt.Equals, uint8(0xCA))
}

// Validates: REQ-NF-06
func TestStreamReaderPanicOnEOF(t *testing.T) {
	c := qt.New(t)
	data := []byte{0x42}
	sr := newStreamReader(bytes.NewReader(data))

	// First read succeeds.
	sr.read1()

	// Second read should trigger panic(errStop).
	c.Assert(func() { sr.read1() }, qt.PanicMatches, "videometa: stop")
	// io.ReadFull returns io.EOF (not ErrUnexpectedEOF) when zero bytes were read.
	c.Assert(sr.readErr, qt.ErrorIs, io.EOF)
}

// Validates: ARCH-IO-02
func TestStreamReaderByteOrderSwitch(t *testing.T) {
	c := qt.New(t)
	// 0x0100 big-endian = 256, little-endian = 1.
	data := []byte{0x01, 0x00, 0x01, 0x00}
	sr := newStreamReader(bytes.NewReader(data))

	sr.byteOrder = binary.BigEndian
	c.Assert(sr.read2(), qt.Equals, uint16(256))

	sr.byteOrder = binary.LittleEndian
	c.Assert(sr.read2(), qt.Equals, uint16(1))
}

// Validates: REQ-NF-06
func TestStreamReaderReadBytesTooLarge(t *testing.T) {
	c := qt.New(t)
	sr := newStreamReader(bytes.NewReader(make([]byte, 100)))

	// Requesting > 10MB should panic.
	c.Assert(func() { sr.readBytes(11 << 20) }, qt.PanicMatches, "videometa: stop")
}

// Validates: REQ-NF-01
func TestFourCCString(t *testing.T) {
	c := qt.New(t)
	fcc := fourCC{'m', 'o', 'o', 'v'}
	c.Assert(fcc.String(), qt.Equals, "moov")
}

// readerOnly wraps an io.ReadSeeker to expose only io.Reader.
type readerOnly struct {
	r io.Reader
}

func (ro readerOnly) Read(p []byte) (int, error) {
	return ro.r.Read(p)
}
