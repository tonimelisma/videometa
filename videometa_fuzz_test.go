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
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		committedSonyFixture,
		committedAppleFixture,
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}
	f.Add([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'})
	f.Add([]byte{0, 0, 0, 8, 'f', 't'})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, err := Decode(Options{
			R:       r,
			Sources: QUICKTIME | VENDOR | CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
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
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		committedSonyFixture,
		committedAppleFixture,
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}
	f.Add([]byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'})
	f.Add([]byte{0, 0, 0, 8, 'f', 't'})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, err := DecodeAll(Options{
			R:       r,
			Sources: QUICKTIME | VENDOR | CONFIG,
		})
		if err != nil && !IsInvalidFormat(err) {
			t.Errorf("expected InvalidFormatError, got: %T: %v", err, err)
		}
	})
}
