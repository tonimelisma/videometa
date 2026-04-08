//go:build !race

package videometa

import (
	"os"
	"sort"
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
			sources: QUICKTIME | VENDOR | CONFIG,
			ceiling: 500 * time.Microsecond,
		},
		{
			name:    "exiftool_quicktime.mov",
			path:    "testdata/exiftool_quicktime.mov",
			sources: QUICKTIME | VENDOR | CONFIG,
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
			data, err := os.ReadFile(tt.path)
			c.Assert(err, qt.IsNil)

			// Measure decoder latency, not filesystem overhead. Benchmarks use the
			// same in-memory reader shape, which keeps this guard aligned with the
			// documented performance target.
			samples := make([]time.Duration, 0, 5)
			for i := 0; i < 6; i++ {
				r := newBytesReadSeeker(data)
				start := time.Now()
				_, err = Decode(Options{
					R:       r,
					Sources: tt.sources,
					HandleTag: func(ti TagInfo) error {
						return nil
					},
				})
				elapsed := time.Since(start)
				c.Assert(err, qt.IsNil)

				if i == 0 {
					continue // Warmup.
				}
				samples = append(samples, elapsed)
			}

			sort.Slice(samples, func(i, j int) bool {
				return samples[i] < samples[j]
			})
			median := samples[len(samples)/2]
			c.Assert(median < tt.ceiling, qt.IsTrue,
				qt.Commentf("median decode took %v, expected < %v (samples=%v)", median, tt.ceiling, samples))
		})
	}
}
