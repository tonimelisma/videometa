package videometa

// decodeMakerNotes dispatches MakerNotes data to manufacturer-specific decoders.
// MakerNotes are proprietary binary blobs embedded in EXIF tag 0x927C. Each
// manufacturer uses a different format (Apple, Canon, Sony, etc.).
//
// Without real device test files to validate against exiftool, implementing
// full tag tables would be guessing. This stub logs a warning and skips.
func (d *videoDecoderMP4) decodeMakerNotes(data []byte) {
	if len(data) == 0 {
		return
	}
	if d.opts.Warnf != nil {
		d.opts.Warnf("decode makernotes: skipping %d bytes (not yet implemented)", len(data))
	}
}
