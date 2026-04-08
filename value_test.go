package videometa

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestCoerceNumericTagValue(t *testing.T) {
	c := qt.New(t)

	v, ok := coerceNumericTagValue(int32(42))
	c.Assert(ok, qt.IsTrue)
	c.Assert(v, qt.Equals, 42.0)

	_, ok = coerceNumericTagValue("42")
	c.Assert(ok, qt.IsFalse)
}
