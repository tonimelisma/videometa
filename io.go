package videometa

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// errStop is the sentinel value used with panic-based control flow.
// Decoders panic(errStop) on read errors; Decode() recovers it.
var errStop = fmt.Errorf("videometa: stop")

// streamReader wraps an io.Reader with byte-order-aware binary read
// methods. It uses panic-based control flow: read errors trigger
// panic(errStop), recovered at the Decode() boundary.
type streamReader struct {
	r            io.Reader
	rs           io.ReadSeeker // Non-nil when the reader supports seeking.
	byteOrder    binary.ByteOrder
	buf          [8]byte // Scratch buffer for small reads.
	readErr      error
	readerOffset int64 // Tracked manually when rs is nil.
	canSeek      bool
}

// newStreamReader creates a streamReader from any io.Reader.
// If r implements io.ReadSeeker, seeking is enabled.
func newStreamReader(r io.Reader) *streamReader {
	sr := &streamReader{
		byteOrder: binary.BigEndian, // ISOBMFF default.
	}
	if rs, ok := r.(io.ReadSeeker); ok {
		sr.r = r
		sr.rs = rs
		sr.canSeek = true
	} else {
		// Wrap plain readers in a buffered reader for efficiency.
		sr.r = bufio.NewReaderSize(r, 4096)
		sr.canSeek = false
	}
	return sr
}

// stop records the error and panics with errStop.
func (sr *streamReader) stop(err error) {
	sr.readErr = err
	panic(errStop)
}

// readFull reads exactly len(p) bytes or stops.
func (sr *streamReader) readFull(p []byte) {
	n, err := io.ReadFull(sr.r, p)
	sr.readerOffset += int64(n)
	if err != nil {
		sr.stop(err)
	}
}

// read1 reads a single byte.
func (sr *streamReader) read1() uint8 {
	sr.readFull(sr.buf[:1])
	return sr.buf[0]
}

// read2 reads a 2-byte unsigned integer using the current byte order.
func (sr *streamReader) read2() uint16 {
	sr.readFull(sr.buf[:2])
	return sr.byteOrder.Uint16(sr.buf[:2])
}

// read4 reads a 4-byte unsigned integer using the current byte order.
func (sr *streamReader) read4() uint32 {
	sr.readFull(sr.buf[:4])
	return sr.byteOrder.Uint32(sr.buf[:4])
}

// read4s reads a 4-byte signed integer using the current byte order.
func (sr *streamReader) read4s() int32 {
	return int32(sr.read4())
}

// read8 reads an 8-byte unsigned integer using the current byte order.
func (sr *streamReader) read8() uint64 {
	sr.readFull(sr.buf[:8])
	return sr.byteOrder.Uint64(sr.buf[:8])
}

// readBytes reads exactly n bytes into a new slice.
func (sr *streamReader) readBytes(n int) []byte {
	if n <= 0 {
		return nil
	}
	// Fuzz defense: reject absurd allocations.
	const maxAlloc = 10 << 20 // 10 MB
	if n > maxAlloc {
		sr.stop(fmt.Errorf("videometa: allocation too large: %d bytes", n))
	}
	b := make([]byte, n)
	sr.readFull(b)
	return b
}

// readBytesInto reads exactly len(p) bytes into the provided buffer.
func (sr *streamReader) readBytesInto(p []byte) {
	sr.readFull(p)
}

// readFourCC reads 4 bytes as a fourCC code.
func (sr *streamReader) readFourCC() fourCC {
	var fcc fourCC
	sr.readFull(fcc[:])
	return fcc
}

// pos returns the current reader offset. For seekable readers this uses
// Seek; for non-seekable readers it returns the tracked offset.
func (sr *streamReader) pos() int64 {
	if sr.canSeek {
		p, err := sr.rs.Seek(0, io.SeekCurrent)
		if err != nil {
			sr.stop(err)
		}
		return p
	}
	return sr.readerOffset
}

// seek moves to an absolute position. Only works with seekable readers.
func (sr *streamReader) seek(offset int64) {
	if !sr.canSeek {
		sr.stop(fmt.Errorf("videometa: seek not supported on io.Reader"))
	}
	_, err := sr.rs.Seek(offset, io.SeekStart)
	if err != nil {
		sr.stop(err)
	}
	sr.readerOffset = offset
}

// skip advances the reader by n bytes. Uses seeking when available,
// otherwise reads and discards.
func (sr *streamReader) skip(n int64) {
	if n <= 0 {
		return
	}
	if sr.canSeek {
		_, err := sr.rs.Seek(n, io.SeekCurrent)
		if err != nil {
			sr.stop(err)
		}
		sr.readerOffset += n
		return
	}
	// Fallback: read and discard.
	written, err := io.CopyN(io.Discard, sr.r, n)
	sr.readerOffset += written
	if err != nil {
		sr.stop(err)
	}
}

// preservePos saves the current position, runs fn, then restores position.
// Only works with seekable readers.
func (sr *streamReader) preservePos(fn func()) {
	saved := sr.pos()
	fn()
	sr.seek(saved)
}

// bufferedReader reads length bytes from the current stream position into a
// pooled buffer and returns a ReadSeeker over that data. The caller MUST
// call Close() on the returned value to return the buffer to the pool.
func (sr *streamReader) bufferedReader(length int) *readerCloser {
	const maxBufferedRead = 10 << 20 // 10 MB
	if length > maxBufferedRead {
		sr.stop(fmt.Errorf("videometa: buffered read too large: %d bytes", length))
	}

	bp := bytesReaderPool.Get().(*bytesReaderPoolItem)
	if cap(bp.buf) < length {
		bp.buf = make([]byte, length)
	} else {
		bp.buf = bp.buf[:length]
	}
	sr.readFull(bp.buf)
	bp.reader.Reset(bp.buf)
	return &readerCloser{
		ReadSeeker: bp.reader,
		poolItem:   bp,
	}
}

// readerCloser wraps a ReadSeeker with a Close method that returns
// the underlying buffer to the pool.
type readerCloser struct {
	io.ReadSeeker
	poolItem *bytesReaderPoolItem
}

// Close returns the buffer to the pool.
func (rc *readerCloser) Close() error {
	if rc.poolItem != nil {
		bytesReaderPool.Put(rc.poolItem)
		rc.poolItem = nil
	}
	return nil
}

type bytesReaderPoolItem struct {
	buf    []byte
	reader *bytes.Reader
}

var bytesReaderPool = sync.Pool{
	New: func() any {
		return &bytesReaderPoolItem{
			buf:    make([]byte, 0, 4096),
			reader: bytes.NewReader(nil),
		}
	},
}

// fourCC is a 4-byte FourCC code used in ISOBMFF.
type fourCC [4]byte

// String returns the fourCC as a printable string.
func (f fourCC) String() string {
	return string(f[:])
}
