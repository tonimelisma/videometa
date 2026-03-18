package videometa

import (
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-EXIF-05
func TestRatUint32(t *testing.T) {
	c := qt.New(t)

	r, err := NewRat[uint32](6, 4)
	c.Assert(err, qt.IsNil)
	c.Assert(r.Num(), qt.Equals, uint32(3))
	c.Assert(r.Den(), qt.Equals, uint32(2))
	c.Assert(r.Float64(), qt.Equals, 1.5)
	c.Assert(r.String(), qt.Equals, "3/2")
}

// Validates: REQ-EXIF-05
func TestRatInt32(t *testing.T) {
	c := qt.New(t)

	r, err := NewRat[int32](-6, 4)
	c.Assert(err, qt.IsNil)
	c.Assert(r.Num(), qt.Equals, int32(-3))
	c.Assert(r.Den(), qt.Equals, int32(2))
	c.Assert(r.Float64(), qt.Equals, -1.5)
}

// Validates: REQ-EXIF-05
func TestRatDenOne(t *testing.T) {
	c := qt.New(t)

	r, err := NewRat[uint32](42, 1)
	c.Assert(err, qt.IsNil)
	c.Assert(r.String(), qt.Equals, "42")
}

// Validates: REQ-NF-06
func TestRatZeroDen(t *testing.T) {
	c := qt.New(t)

	_, err := NewRat[uint32](1, 0)
	c.Assert(err, qt.IsNotNil)
}

// Validates: REQ-EXIF-05
func TestRatMarshalText(t *testing.T) {
	c := qt.New(t)

	r, err := NewRat[uint32](3, 2)
	c.Assert(err, qt.IsNil)

	text, err := r.MarshalText()
	c.Assert(err, qt.IsNil)
	c.Assert(string(text), qt.Equals, "3/2")

	r2, err := NewRat[uint32](1, 1)
	c.Assert(err, qt.IsNil)
	err = r2.UnmarshalText(text)
	c.Assert(err, qt.IsNil)
	c.Assert(r2.Num(), qt.Equals, uint32(3))
	c.Assert(r2.Den(), qt.Equals, uint32(2))
}

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

// Validates: REQ-EXIF-05
func TestConvertAPEXToFNumber(t *testing.T) {
	c := qt.New(t)
	// APEX 5.0 should give f/5.657 (2^(5/2) ≈ 5.657)
	f := convertAPEXToFNumber(5.0)
	c.Assert(math.Abs(f-5.6568) < 0.001, qt.IsTrue)
}

// Validates: REQ-EXIF-05
func TestConvertAPEXToSeconds(t *testing.T) {
	c := qt.New(t)
	// APEX 6.0 should give 1/64 seconds (2^-6 ≈ 0.015625)
	s := convertAPEXToSeconds(6.0)
	c.Assert(math.Abs(s-0.015625) < 0.0001, qt.IsTrue)
}

// Validates: REQ-EXIF-05
func TestConvertDegreesToDecimal(t *testing.T) {
	c := qt.New(t)
	// 34°3'33" = 34.059166...
	d := convertDegreesToDecimal(34, 3, 33)
	c.Assert(math.Abs(d-34.059166) < 0.001, qt.IsTrue)
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

func TestMatrixToRotation(t *testing.T) {
	c := qt.New(t)

	// Identity (0°).
	c.Assert(matrixToRotation([9]int32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 0)
	// 90° CW.
	c.Assert(matrixToRotation([9]int32{0, 0x10000, 0, -0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 90)
	// 180°.
	c.Assert(matrixToRotation([9]int32{-0x10000, 0, 0, 0, -0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 180)
	// 270° CW.
	c.Assert(matrixToRotation([9]int32{0, -0x10000, 0, 0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 270)
	// Non-standard → 0.
	c.Assert(matrixToRotation([9]int32{0, 0, 0, 0, 0, 0, 0, 0, 0}), qt.Equals, 0)
}

func TestCapitalizeFirst(t *testing.T) {
	c := qt.New(t)

	c.Assert(capitalizeFirst("hello"), qt.Equals, "Hello")
	c.Assert(capitalizeFirst("Hello"), qt.Equals, "Hello")
	c.Assert(capitalizeFirst(""), qt.Equals, "")
	c.Assert(capitalizeFirst("a"), qt.Equals, "A")
}
