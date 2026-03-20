package videometa

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// InvalidFormatError indicates malformed input data.
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

// Rat represents a rational number (numerator/denominator).
type Rat[T int32 | uint32] interface {
	Num() T
	Den() T
	Float64() float64
	String() string
	encoding.TextMarshaler
	encoding.TextUnmarshaler
}

type rat[T int32 | uint32] struct {
	num, den T
}

// NewRat creates a new rational number, reduced to lowest terms.
func NewRat[T int32 | uint32](num, den T) (Rat[T], error) {
	if den == 0 {
		return nil, fmt.Errorf("videometa: zero denominator")
	}
	g := gcd(num, den)
	return &rat[T]{num: num / g, den: den / g}, nil
}

func (r *rat[T]) Num() T { return r.num }
func (r *rat[T]) Den() T { return r.den }
func (r *rat[T]) Float64() float64 {
	if r.den == 0 {
		return 0
	}
	return float64(r.num) / float64(r.den)
}

func (r *rat[T]) String() string {
	if r.den == 1 {
		return fmt.Sprintf("%d", r.num)
	}
	return fmt.Sprintf("%d/%d", r.num, r.den)
}

func (r *rat[T]) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *rat[T]) UnmarshalText(text []byte) error {
	s := string(text)
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		num, err := strconv.ParseInt(s[:idx], 10, 64)
		if err != nil {
			return fmt.Errorf("videometa: parse rational numerator: %w", err)
		}
		den, err := strconv.ParseInt(s[idx+1:], 10, 64)
		if err != nil {
			return fmt.Errorf("videometa: parse rational denominator: %w", err)
		}
		r.num = T(num)
		r.den = T(den)
	} else {
		num, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("videometa: parse rational: %w", err)
		}
		r.num = T(num)
		r.den = 1
	}
	return nil
}

func gcd[T int32 | uint32](a, b T) T {
	// Work with absolute values for signed types.
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// parseISO6709 parses an ISO 6709 coordinate string like "+34.0592-118.4460+042.938/"
// into latitude and longitude in decimal degrees.
func parseISO6709(s string) (lat, lon float64, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if len(s) == 0 {
		return 0, 0, fmt.Errorf("videometa: empty ISO 6709 string")
	}

	// Find the sign characters that delimit lat/lon/alt.
	// Format: ±DD.DD±DDD.DD±AAA.AA or ±DDMM.MM±DDDMM.MM±AAA.AA etc.
	// The first character is always the latitude sign.
	// We need to find the second sign (longitude) and optional third (altitude).
	parts := splitISO6709(s)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("videometa: parse ISO 6709: need at least lat+lon, got %q", s)
	}

	lat, err = parseISO6709Coord(parts[0], true)
	if err != nil {
		return 0, 0, fmt.Errorf("videometa: parse ISO 6709 latitude: %w", err)
	}
	lon, err = parseISO6709Coord(parts[1], false)
	if err != nil {
		return 0, 0, fmt.Errorf("videometa: parse ISO 6709 longitude: %w", err)
	}
	return lat, lon, nil
}

// splitISO6709 splits an ISO 6709 string into signed components.
// Example: "+34.0592-118.4460+042.938" → ["+34.0592", "-118.4460", "+042.938"]
func splitISO6709(s string) []string {
	var parts []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			parts = append(parts, s[start:i])
			start = i
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// parseISO6709Coord parses a single signed coordinate value.
// isLat determines whether to expect 2-digit (lat) or 3-digit (lon) degrees.
func parseISO6709Coord(s string, isLat bool) (float64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty coordinate")
	}

	sign := 1.0
	switch s[0] {
	case '-':
		sign = -1.0
		s = s[1:]
	case '+':
		s = s[1:]
	}

	// Try parsing as plain decimal first.
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return sign * v, nil
	}

	// Try DMS format: DDMM.MM or DDDMM.MM or DDMMSS.SS or DDDMMSS.SS
	degDigits := 2
	if !isLat {
		degDigits = 3
	}

	if len(s) < degDigits+2 {
		return 0, fmt.Errorf("coordinate too short: %q", s)
	}

	deg, err := strconv.ParseFloat(s[:degDigits], 64)
	if err != nil {
		return 0, err
	}
	rest := s[degDigits:]

	// Check if remaining has seconds (length > 4 digits before decimal)
	dotIdx := strings.IndexByte(rest, '.')
	intPart := rest
	if dotIdx >= 0 {
		intPart = rest[:dotIdx]
	}

	if len(intPart) <= 2 {
		// MM.MM format
		min, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, err
		}
		return sign * (deg + min/60), nil
	}
	// MMSS.SS format
	min, err := strconv.ParseFloat(rest[:2], 64)
	if err != nil {
		return 0, err
	}
	sec, err := strconv.ParseFloat(rest[2:], 64)
	if err != nil {
		return 0, err
	}
	return sign * (deg + min/60 + sec/3600), nil
}

// convertDateToExiftool converts an ISO 8601 date string to exiftool's format.
// "2024-06-15T10:30:00-0700" → "2024:06:15 10:30:00-07:00"
// Returns empty string if unparseable.
func convertDateToExiftool(s string) string {
	t, err := parseTimeString(s)
	if err != nil {
		return ""
	}
	_, offset := t.Zone()
	if offset == 0 && t.Location() == time.UTC {
		return t.Format("2006:01:02 15:04:05")
	}
	return t.Format("2006:01:02 15:04:05-07:00")
}

// convertISO6709ToExiftool converts an ISO 6709 coordinate string like
// "+34.0592-118.4460+042.938/" to exiftool's space-separated format
// "34.0592 -118.446 42.938".
func convertISO6709ToExiftool(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	parts := splitISO6709(s)
	if len(parts) < 2 {
		return s
	}

	var result []string
	for _, p := range parts {
		sign := ""
		v := p
		if len(v) > 0 && (v[0] == '+' || v[0] == '-') {
			if v[0] == '-' {
				sign = "-"
			}
			v = v[1:]
		}
		// Parse as float to normalize (removes leading zeros, trailing zeros).
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			result = append(result, p) // Keep original if unparseable.
			continue
		}
		result = append(result, sign+strconv.FormatFloat(f, 'f', -1, 64))
	}

	return strings.Join(result, " ")
}

// parseGPSCoordinatesString parses exiftool-formatted GPS coordinates
// (space-separated: "lat lon [alt]") or ISO 6709 format.
func parseGPSCoordinatesString(s string) (lat, lon float64, err error) {
	s = strings.TrimSpace(s)
	// Try space-separated format first (exiftool output format).
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lon, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lon, nil
		}
	}
	// Fall back to ISO 6709.
	return parseISO6709(s)
}

// parseGPSAltitudeFromString extracts altitude from a space-separated GPS string.
// Returns (altitude, true) if found, or (0, false) if not.
func parseGPSAltitudeFromString(s string) (float64, bool) {
	parts := strings.Fields(s)
	if len(parts) >= 3 {
		alt, err := strconv.ParseFloat(parts[2], 64)
		if err == nil {
			return alt, true
		}
	}
	return 0, false
}

// toFloat64 converts a numeric value to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint8:
		return float64(n), true
	default:
		return 0, false
	}
}

// toString converts a value to string.
func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// printableString returns s with non-printable characters removed and trimmed.
func printableString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// trimNulls removes trailing null bytes from a byte slice.
func trimNulls(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

// parseAnyDateTime attempts to parse a value as a time.Time using multiple
// strategies: direct time.Time, EXIF-style strings, ISO 8601, QuickTime dates.
func parseAnyDateTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return time.Time{}, fmt.Errorf("zero time")
		}
		return t, nil
	case string:
		return parseTimeString(t)
	default:
		return time.Time{}, fmt.Errorf("unsupported type %T for date/time", v)
	}
}

// parseTimeString tries multiple date/time formats used by EXIF, XMP, and QuickTime.
func parseTimeString(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000:00:00 00:00:00" {
		return time.Time{}, fmt.Errorf("empty or zero date")
	}

	formats := []string{
		"2006:01:02 15:04:05",       // EXIF
		"2006:01:02 15:04:05-07:00", // EXIF with timezone
		"2006:01:02 15:04:05Z07:00", // EXIF with Z timezone
		"2006-01-02T15:04:05-07:00", // ISO 8601
		"2006-01-02T15:04:05Z07:00", // ISO 8601 with Z
		"2006-01-02T15:04:05-0700",  // QuickTime
		"2006-01-02T15:04:05Z",      // ISO 8601 UTC
		"2006-01-02T15:04:05",       // ISO 8601 no TZ
		"2006-01-02 15:04:05",       // Simple
		"2006:01:02",                // Date only
		"2006-01-02",                // Date only ISO
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
}

// convertAPEXToFNumber converts an APEX aperture value to an f-number.
func convertAPEXToFNumber(apex float64) float64 {
	return math.Pow(2, apex/2)
}

// convertAPEXToSeconds converts an APEX shutter speed value to seconds.
func convertAPEXToSeconds(apex float64) float64 {
	return math.Pow(2, -apex)
}

// convertDegreesToDecimal converts GPS DMS (degrees, minutes, seconds as
// three rationals) to decimal degrees.
func convertDegreesToDecimal(degrees, minutes, seconds float64) float64 {
	return degrees + minutes/60 + seconds/3600
}
