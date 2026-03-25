//go:build !race

package videometa

import (
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

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
			ceiling: 500 * time.Microsecond,
		},
		{
			name:    "exiftool_quicktime.mov",
			path:    "testdata/exiftool_quicktime.mov",
			sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES,
			ceiling: 500 * time.Microsecond,
		},
		{
			name:    "with_audio.mp4",
			path:    "testdata/with_audio.mp4",
			sources: QUICKTIME | CONFIG,
			ceiling: 500 * time.Microsecond,
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
