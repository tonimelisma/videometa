package videometa

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-QT-01
func TestDecodeUTF16BE(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name   string
		input  []byte
		expect string
	}{
		{"ascii", []byte{0x00, 'H', 0x00, 'i'}, "Hi"},
		{"with null terminator", []byte{0x00, 'A', 0x00, 0x00, 0x00, 'B'}, "A"},
		{"empty", []byte{}, ""},
		{"odd length truncated", []byte{0x00, 'X', 0x00}, "X"},
		{"unicode", []byte{0x00, 0xE9}, "\u00e9"}, // é
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(decodeUTF16BE(tt.input), qt.Equals, tt.expect)
		})
	}
}

// Validates: REQ-QT-01
func TestDecodeLocale(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name   string
		locale uint32
		expect string
	}{
		{"zero", 0x00000000, ""},
		{"eng-US", 0x555315c7, "eng-US"},
		{"fra-FR", 0x46521a41, "fra-FR"},
		{"lang only no country", 0x000015c7, "eng"},
		{"undetermined", 0x00000000, ""},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(decodeLocale(tt.locale), qt.Equals, tt.expect)
		})
	}
}

// Validates: REQ-QT-02, REQ-QT-03
func TestFreeformToTagName(t *testing.T) {
	c := qt.New(t)

	// Known mapping.
	c.Assert(freeformToTagName("com.apple.quicktime", "make"), qt.Equals, "Make")
	c.Assert(freeformToTagName("com.apple.quicktime", "camera.lens_model"), qt.Equals, "LensModel")

	// Unknown key.
	c.Assert(freeformToTagName("com.apple.quicktime", "unknown.key"), qt.Equals, "")

	// Wrong vendor.
	c.Assert(freeformToTagName("com.google.android", "make"), qt.Equals, "")
}
