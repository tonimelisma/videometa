package videometa

import "strings"

// sanitizeMetadataString drops non-printable characters and trims whitespace
// from text extracted out of video metadata payloads.
func sanitizeMetadataString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 32 && r != 127 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// trimTrailingNulls removes null padding from metadata payload strings.
func trimTrailingNulls(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}
