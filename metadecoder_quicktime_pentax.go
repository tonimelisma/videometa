package videometa

import "encoding/binary"

// decodePentaxTAGS parses the Pentax TAGS atom found in QuickTime MOV files
// recorded by Pentax digital cameras. The binary data uses little-endian byte
// order with fixed offsets for each field (derived from exiftool QuickTime.pm).
func (d *videoDecoderMP4) decodePentaxTAGS(data []byte) {
	le := binary.LittleEndian

	// Make: null-terminated string at offset 0, 24 bytes.
	if len(data) >= 24 {
		make_ := printableString(string(trimNulls(data[0:24])))
		if make_ != "" {
			d.emitPentaxQuickTimeTag("Make", make_)
		}
	}

	// ExposureTime: int32u at offset 0x26, value conversion: 10 / raw.
	if len(data) >= 0x2A {
		raw := le.Uint32(data[0x26:0x2A])
		if raw > 0 {
			d.emitPentaxQuickTimeTag("ExposureTime", 10.0/float64(raw))
		}
	}

	// FNumber: rational64u (2×int32u) at offset 0x2A.
	if len(data) >= 0x32 {
		num := le.Uint32(data[0x2A:0x2E])
		den := le.Uint32(data[0x2E:0x32])
		if den > 0 {
			d.emitPentaxQuickTimeTag("FNumber", float64(num)/float64(den))
		}
	}

	// ExposureCompensation: rational64s (2×int32s) at offset 0x32.
	if len(data) >= 0x3A {
		num := int32(le.Uint32(data[0x32:0x36]))
		den := int32(le.Uint32(data[0x36:0x3A]))
		if den != 0 {
			d.emitPentaxQuickTimeTag("ExposureCompensation", float64(num)/float64(den))
		}
	}

	// WhiteBalance: int16u at offset 0x44.
	if len(data) >= 0x46 {
		d.emitPentaxQuickTimeTag("WhiteBalance", int(le.Uint16(data[0x44:0x46])))
	}

	// FocalLength: rational64u (2×int32u) at offset 0x48.
	if len(data) >= 0x50 {
		num := le.Uint32(data[0x48:0x4C])
		den := le.Uint32(data[0x4C:0x50])
		if den > 0 {
			d.emitPentaxQuickTimeTag("FocalLength", float64(num)/float64(den))
		}
	}

	// ISO: int16u at offset 0xAF.
	if len(data) >= 0xB1 {
		d.emitPentaxQuickTimeTag("ISO", int(le.Uint16(data[0xAF:0xB1])))
	}
}

func (d *videoDecoderMP4) emitPentaxQuickTimeTag(name string, value any) {
	d.emitTag(TagInfo{
		Source:    QUICKTIME,
		Tag:       name,
		Namespace: "QuickTime",
		Value:     value,
	})
}
