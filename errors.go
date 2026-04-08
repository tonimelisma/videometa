package videometa

import (
	"errors"
	"fmt"
)

// InvalidFormatError indicates malformed container input data.
type InvalidFormatError struct {
	Err error
}

func (e *InvalidFormatError) Error() string {
	return fmt.Sprintf("videometa: invalid format: %v", e.Err)
}

func (e *InvalidFormatError) Unwrap() error {
	return e.Err
}

// IsInvalidFormat reports whether err is an InvalidFormatError.
func IsInvalidFormat(err error) bool {
	var target *InvalidFormatError
	return errors.As(err, &target)
}

func newInvalidFormatErrorf(format string, args ...any) error {
	return &InvalidFormatError{Err: fmt.Errorf(format, args...)}
}
