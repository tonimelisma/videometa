package videometa

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-06
func TestSanitizeMetadataString(t *testing.T) {
	c := qt.New(t)
	c.Assert(sanitizeMetadataString("hello\x00\x01world"), qt.Equals, "helloworld")
	c.Assert(sanitizeMetadataString("  clean  "), qt.Equals, "clean")
}

// Validates: REQ-NF-06
func TestTrimTrailingNulls(t *testing.T) {
	c := qt.New(t)
	c.Assert(trimTrailingNulls([]byte("hello\x00\x00")), qt.DeepEquals, []byte("hello"))
	c.Assert(trimTrailingNulls([]byte("hello")), qt.DeepEquals, []byte("hello"))
	c.Assert(trimTrailingNulls([]byte{0, 0}), qt.DeepEquals, []byte{})
}
