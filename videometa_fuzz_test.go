package videometa

import (
	"bytes"
	"os"
	"testing"
)

// Validates: REQ-NF-05
func FuzzDecodeMP4(f *testing.F) {
	// Seed corpus from test files.
	seeds := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/truncated.mp4",
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic on any input.
		r := bytes.NewReader(data)
		Decode(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
	})
}

// Validates: REQ-NF-05
func FuzzDecodeAllMP4(f *testing.F) {
	seeds := []string{
		"testdata/minimal.mp4",
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		DecodeAll(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
		})
	})
}
