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
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		"testdata/sony_a6700.mp4",
		"testdata/apple.mov",
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
		_, err := Decode(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
		// All errors from malformed input must be InvalidFormatError.
		if err != nil && !IsInvalidFormat(err) {
			t.Errorf("expected InvalidFormatError, got: %T: %v", err, err)
		}
	})
}

// Validates: REQ-NF-05
func FuzzDecodeAllMP4(f *testing.F) {
	seeds := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/truncated.mp4",
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		"testdata/sony_a6700.mp4",
		"testdata/apple.mov",
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
		_, _, err := DecodeAll(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
		})
		// All errors from malformed input must be InvalidFormatError.
		if err != nil && !IsInvalidFormat(err) {
			t.Errorf("expected InvalidFormatError, got: %T: %v", err, err)
		}
	})
}
