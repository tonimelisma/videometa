package videometa

import (
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-06
func TestInvalidFormatError(t *testing.T) {
	c := qt.New(t)

	err := newInvalidFormatErrorf("bad box at offset %d", 42)
	c.Assert(IsInvalidFormat(err), qt.IsTrue)
	c.Assert(err.Error(), qt.Matches, `.*invalid format.*42.*`)

	c.Assert(IsInvalidFormat(nil), qt.IsFalse)
}

// Validates: REQ-QT-06
func TestParseISO6709(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name    string
		input   string
		wantLat float64
		wantLon float64
		wantErr bool
	}{
		{
			name:    "decimal with altitude",
			input:   "+34.0592-118.4460+042.938/",
			wantLat: 34.0592,
			wantLon: -118.4460,
		},
		{
			name:    "simple decimal",
			input:   "+48.8566+002.3522/",
			wantLat: 48.8566,
			wantLon: 2.3522,
		},
		{
			name:    "negative both",
			input:   "-33.8688+151.2093/",
			wantLat: -33.8688,
			wantLon: 151.2093,
		},
		{
			name:    "no trailing slash",
			input:   "+40.7128-074.0060",
			wantLat: 40.7128,
			wantLon: -74.0060,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			lat, lon, err := parseISO6709(tt.input)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
			c.Assert(math.Abs(lat-tt.wantLat) < 0.0001, qt.IsTrue, qt.Commentf("lat: got %f, want %f", lat, tt.wantLat))
			c.Assert(math.Abs(lon-tt.wantLon) < 0.0001, qt.IsTrue, qt.Commentf("lon: got %f, want %f", lon, tt.wantLon))
		})
	}
}

// Validates: REQ-NF-06
func TestPrintableString(t *testing.T) {
	c := qt.New(t)
	c.Assert(printableString("hello\x00\x01world"), qt.Equals, "helloworld")
	c.Assert(printableString("  clean  "), qt.Equals, "clean")
}

// Validates: REQ-NF-06
func TestTrimNulls(t *testing.T) {
	c := qt.New(t)
	c.Assert(trimNulls([]byte("hello\x00\x00")), qt.DeepEquals, []byte("hello"))
	c.Assert(trimNulls([]byte("hello")), qt.DeepEquals, []byte("hello"))
	c.Assert(trimNulls([]byte{0, 0}), qt.DeepEquals, []byte{})
}

// Validates: REQ-CFG-03
func TestMatrixToRotation(t *testing.T) {
	c := qt.New(t)

	c.Assert(matrixToRotation([9]int32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 0)
	c.Assert(matrixToRotation([9]int32{0, 0x10000, 0, -0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 90)
	c.Assert(matrixToRotation([9]int32{-0x10000, 0, 0, 0, -0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 180)
	c.Assert(matrixToRotation([9]int32{0, -0x10000, 0, 0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 270)
	c.Assert(matrixToRotation([9]int32{0, 0, 0, 0, 0, 0, 0, 0, 0}), qt.Equals, 0)
}
