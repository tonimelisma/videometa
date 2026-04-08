package videometa

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-CFG-03
func TestMatrixToRotation(t *testing.T) {
	c := qt.New(t)

	c.Assert(matrixToRotation([9]int32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 0)
	c.Assert(matrixToRotation([9]int32{0, 0x10000, 0, -0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 90)
	c.Assert(matrixToRotation([9]int32{-0x10000, 0, 0, 0, -0x10000, 0, 0, 0, 0x40000000}), qt.Equals, 180)
	c.Assert(matrixToRotation([9]int32{0, -0x10000, 0, 0x10000, 0, 0, 0, 0, 0x40000000}), qt.Equals, 270)
	c.Assert(matrixToRotation([9]int32{0, 0, 0, 0, 0, 0, 0, 0, 0}), qt.Equals, 0)
}
