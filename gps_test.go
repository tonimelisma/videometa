package videometa

import (
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

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

// Validates: REQ-API-13
func TestParseVideoGPSCoordinates(t *testing.T) {
	c := qt.New(t)

	lat, lon, err := parseVideoGPSCoordinates("34.0592 -118.446 42.938")
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-34.0592) < 0.0001, qt.IsTrue)
	c.Assert(math.Abs(lon-(-118.446)) < 0.0001, qt.IsTrue)
}

// Validates: REQ-API-13
func TestParseVideoGPSAltitude(t *testing.T) {
	c := qt.New(t)

	alt, ok := parseVideoGPSAltitude("34.0592 -118.446 42.938")
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(alt-42.938) < 0.0001, qt.IsTrue)
}

// Validates: REQ-API-13, REQ-VENDOR-01
func TestParseVideoRefCoordinate(t *testing.T) {
	c := qt.New(t)

	lat, err := parseVideoRefCoordinate("29;19;10.922", "N")
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lat-(29+19.0/60+10.922/3600)) < 0.000001, qt.IsTrue)

	lon, err := parseVideoRefCoordinate("103;36;36.925", "W")
	c.Assert(err, qt.IsNil)
	c.Assert(math.Abs(lon-(-(103+36.0/60+36.925/3600))) < 0.000001, qt.IsTrue)
}
