package videometa

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// parseISO6709 parses an ISO 6709 coordinate string like
// "+34.0592-118.4460+042.938/" into decimal latitude and longitude.
func parseISO6709(s string) (lat, lon float64, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if len(s) == 0 {
		return 0, 0, fmt.Errorf("videometa: empty ISO 6709 string")
	}

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

	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return sign * v, nil
	}

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

	dotIdx := strings.IndexByte(rest, '.')
	intPart := rest
	if dotIdx >= 0 {
		intPart = rest[:dotIdx]
	}

	if len(intPart) <= 2 {
		min, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, err
		}
		return sign * (deg + min/60), nil
	}

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

// formatISO6709ForExiftool converts an ISO 6709 coordinate string into
// exiftool's space-separated numeric format.
func formatISO6709ForExiftool(s string) string {
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
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			result = append(result, p)
			continue
		}
		result = append(result, sign+strconv.FormatFloat(f, 'f', -1, 64))
	}

	return strings.Join(result, " ")
}

// parseVideoGPSCoordinates parses either exiftool-formatted space-separated
// coordinates or ISO 6709 coordinates from supported video metadata.
func parseVideoGPSCoordinates(s string) (lat, lon float64, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lon, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lon, nil
		}
	}
	return parseISO6709(s)
}

// parseVideoGPSAltitude extracts an altitude from a supported space-separated
// GPS string if one is present.
func parseVideoGPSAltitude(s string) (float64, bool) {
	parts := strings.Fields(s)
	if len(parts) >= 3 {
		alt, err := strconv.ParseFloat(parts[2], 64)
		if err == nil {
			return alt, true
		}
	}
	return 0, false
}

// parseVideoRefCoordinate parses a DMS coordinate plus hemisphere ref from
// vendor metadata such as Sony NRTM's Latitude/LatitudeRef pairs.
func parseVideoRefCoordinate(value string, ref string) (float64, error) {
	value = strings.TrimSpace(value)
	ref = strings.TrimSpace(strings.ToUpper(ref))
	if value == "" || ref == "" {
		return 0, fmt.Errorf("missing coordinate value or ref")
	}

	if decimal, err := strconv.ParseFloat(value, 64); err == nil {
		switch ref {
		case "S", "W":
			return -math.Abs(decimal), nil
		case "N", "E":
			return math.Abs(decimal), nil
		default:
			return 0, fmt.Errorf("unsupported coordinate ref %q", ref)
		}
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ';' || r == ',' || r == ' '
	})
	if len(parts) != 3 {
		return 0, fmt.Errorf("unsupported coordinate format %q", value)
	}

	deg, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	min, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}
	sec, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	decimal := deg + min/60 + sec/3600
	switch ref {
	case "S", "W":
		return -decimal, nil
	case "N", "E":
		return decimal, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate ref %q", ref)
	}
}
