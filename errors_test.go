package videometa

import (
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
