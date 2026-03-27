//go:build !race

package videometa

import (
	"bytes"
	"os"
	"runtime"
	"runtime/debug"
	"testing"

	qt "github.com/frankban/quicktest"
)

type allocationBudget struct {
	maxAllocs float64
	maxBytes  uint64
}

// Validates: REQ-NF-01, REQ-NF-02
func TestDecodeAllocationBudgets(t *testing.T) {
	c := qt.New(t)

	minimalData, err := os.ReadFile("testdata/minimal.mp4")
	c.Assert(err, qt.IsNil)

	quickTimeData, err := os.ReadFile("testdata/exiftool_quicktime.mov")
	c.Assert(err, qt.IsNil)

	largeMdatData := buildMP4WithLargeMdat(256 << 10)
	largeNRTMData := buildMP4WithSonyNRTMIDAT(bytes.Repeat([]byte("N"), 2<<20))

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Alloc Cam"))
	metaIlocReaderOnly := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	tests := []struct {
		name   string
		budget allocationBudget
		decode func()
	}{
		{
			name:   "decode-all-minimal",
			budget: allocationBudget{maxAllocs: 600, maxBytes: 96 << 10},
			decode: func() {
				_, err := DecodeAll(Options{
					R:       newBytesReadSeeker(minimalData),
					Sources: EXIF | XMP | IPTC | QUICKTIME | VENDOR | CONFIG | COMPOSITE,
				})
				c.Assert(err, qt.IsNil)
			},
		},
		{
			name:   "decode-all-exiftool-quicktime",
			budget: allocationBudget{maxAllocs: 1400, maxBytes: 192 << 10},
			decode: func() {
				_, err := DecodeAll(Options{
					R:       newBytesReadSeeker(quickTimeData),
					Sources: EXIF | XMP | IPTC | QUICKTIME | VENDOR | CONFIG | COMPOSITE,
				})
				c.Assert(err, qt.IsNil)
			},
		},
		{
			name:   "large-mdat-reader-only",
			budget: allocationBudget{maxAllocs: 80, maxBytes: 16 << 10},
			decode: func() {
				_, err := Decode(Options{
					R:         readerOnly{bytes.NewReader(largeMdatData)},
					Sources:   QUICKTIME | CONFIG,
					HandleTag: func(TagInfo) error { return nil },
				})
				c.Assert(err, qt.IsNil)
			},
		},
		{
			name:   "sony-nrtm-large-idat",
			budget: allocationBudget{maxAllocs: 80, maxBytes: 16 << 10},
			decode: func() {
				_, err := Decode(Options{
					R:         newBytesReadSeeker(largeNRTMData),
					Sources:   VENDOR,
					HandleTag: func(TagInfo) error { return nil },
				})
				c.Assert(err, qt.IsNil)
			},
		},
		{
			name:   "meta-iloc-reader-only",
			budget: allocationBudget{maxAllocs: 96, maxBytes: 16 << 10},
			decode: func() {
				_, err := Decode(Options{
					R:         readerOnly{bytes.NewReader(metaIlocReaderOnly)},
					Sources:   EXIF,
					HandleTag: func(TagInfo) error { return nil },
				})
				c.Assert(err, qt.IsNil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			allocs, bytesAllocated := measureAllocations(tt.decode)
			c.Assert(allocs <= tt.budget.maxAllocs, qt.IsTrue,
				qt.Commentf("%s allocated %.0f objects, budget %.0f", tt.name, allocs, tt.budget.maxAllocs))
			c.Assert(bytesAllocated <= tt.budget.maxBytes, qt.IsTrue,
				qt.Commentf("%s allocated %d bytes, budget %d", tt.name, bytesAllocated, tt.budget.maxBytes))
		})
	}
}

func measureAllocations(fn func()) (float64, uint64) {
	allocs := testing.AllocsPerRun(20, fn)

	runtime.GC()
	debug.FreeOSMemory()

	var before runtime.MemStats
	var after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)

	return allocs, after.TotalAlloc - before.TotalAlloc
}
