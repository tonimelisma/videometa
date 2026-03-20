package videometa

import (
	"io"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
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

// Validates: REQ-NF-02
func TestDecodeLatencyTarget(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		sources Source
		ceiling time.Duration
	}{
		{
			name:    "minimal.mp4",
			path:    "testdata/minimal.mp4",
			sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
			ceiling: 2 * time.Millisecond,
		},
		{
			name:    "exiftool_quicktime.mov",
			path:    "testdata/exiftool_quicktime.mov",
			sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES,
			ceiling: 5 * time.Millisecond,
		},
		{
			name:    "with_audio.mp4",
			path:    "testdata/with_audio.mp4",
			sources: QUICKTIME | CONFIG,
			ceiling: 5 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f, err := os.Open(tt.path)
			c.Assert(err, qt.IsNil)
			defer func() { _ = f.Close() }()

			start := time.Now()
			_, err = Decode(Options{
				R:       f,
				Sources: tt.sources,
				HandleTag: func(ti TagInfo) error {
					return nil
				},
			})
			elapsed := time.Since(start)
			c.Assert(err, qt.IsNil)
			c.Assert(elapsed < tt.ceiling, qt.IsTrue,
				qt.Commentf("decode took %v, expected < %v", elapsed, tt.ceiling))
		})
	}
}

// Validates: REQ-NF-05
func TestSeedCorpusDecodesSuccessfully(t *testing.T) {
	// Ensure all committed test files decode without error.
	// Catches regressions where valid files start returning errors.
	files := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			c := qt.New(t)
			f, err := os.Open(path)
			c.Assert(err, qt.IsNil)
			defer func() { _ = f.Close() }()

			tagCount := 0
			_, err = Decode(Options{
				R:       f,
				Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES,
				HandleTag: func(ti TagInfo) error {
					tagCount++
					return nil
				},
			})
			c.Assert(err, qt.IsNil,
				qt.Commentf("valid file %s must decode without error", path))
			c.Assert(tagCount > 0, qt.IsTrue,
				qt.Commentf("valid file %s must produce at least one tag", path))
		})
	}
}
