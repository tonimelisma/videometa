package videometa

import (
	"io"
	"os"
	"testing"
)

// Validates: REQ-NF-02, REQ-NF-03
func BenchmarkDecodeMinimalMP4AllSources(b *testing.B) {
	data, err := os.ReadFile("testdata/minimal.mp4")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _ = Decode(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
	}
}

// Validates: REQ-NF-03
func BenchmarkDecodeMinimalMP4ConfigOnly(b *testing.B) {
	data, err := os.ReadFile("testdata/minimal.mp4")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _ = Decode(Options{
			R:       r,
			Sources: CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
	}
}

// Validates: REQ-NF-03
func BenchmarkDecodeMinimalMP4QuickTime(b *testing.B) {
	data, err := os.ReadFile("testdata/minimal.mp4")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _ = Decode(Options{
			R:       r,
			Sources: QUICKTIME,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
	}
}

// Validates: REQ-NF-03
func BenchmarkDecodeAllMinimalMP4(b *testing.B) {
	data, err := os.ReadFile("testdata/minimal.mp4")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _, _ = DecodeAll(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
		})
	}
}

// Validates: REQ-NF-02, REQ-NF-03
func BenchmarkDecodeExifToolQuickTimeMOV(b *testing.B) {
	data, err := os.ReadFile("testdata/exiftool_quicktime.mov")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _, _ = DecodeAll(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES,
		})
	}
}

// Validates: REQ-NF-03
func BenchmarkDecodeWithAudioMP4(b *testing.B) {
	data, err := os.ReadFile("testdata/with_audio.mp4")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		r := newBytesReadSeeker(data)
		_, _, _ = DecodeAll(Options{
			R:       r,
			Sources: QUICKTIME | CONFIG,
		})
	}
}

// newBytesReadSeeker creates an io.ReadSeeker from a byte slice.
// Separate from test helper to avoid import cycle.
func newBytesReadSeeker(data []byte) *bytesReadSeeker {
	return &bytesReadSeeker{data: data, pos: 0}
}

type bytesReadSeeker struct {
	data []byte
	pos  int
}

func (b *bytesReadSeeker) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		b.pos = int(offset)
	case 1:
		b.pos += int(offset)
	case 2:
		b.pos = len(b.data) + int(offset)
	}
	if b.pos < 0 {
		b.pos = 0
	}
	return int64(b.pos), nil
}
