package videometa

import "math"

type canonMakerNoteState struct {
	focalUnits uint16
}

func (d *videoDecoderMP4) decodeCanonMakerNotes(data []byte, ctx makerNoteContext) {
	md := newMakerNoteDecoder(data, ctx.byteOrder, 0)
	state := &canonMakerNoteState{}

	md.walkIFD(0, func(tagID, _ uint16, _ uint32, value any) {
		switch tagID {
		case 0x0001:
			state.decodeCameraSettings(d, value)
		case 0x0002:
			state.decodeFocalLength(d, value)
		case 0x0004:
			state.decodeShotInfo(d, value)
		case 0x0006:
			if s, ok := value.(string); ok && s != "" {
				d.emitMakerNotesTag("CanonImageType", s)
			}
		case 0x0007:
			if s, ok := value.(string); ok && s != "" {
				d.emitMakerNotesTag("CanonFirmwareVersion", s)
			}
		case 0x0008:
			if v, ok := value.(uint32); ok {
				d.emitMakerNotesTag("FileNumber", v)
			}
		case 0x0009:
			if s, ok := value.(string); ok && s != "" {
				d.emitMakerNotesTag("OwnerName", s)
			}
		case 0x000C:
			if v, ok := value.(uint32); ok {
				d.emitMakerNotesTag("SerialNumber", v)
			}
		}
	})
}

func (s *canonMakerNoteState) decodeCameraSettings(d *videoDecoderMP4, value any) {
	values := coerceUint16Slice(value)
	if len(values) == 0 {
		return
	}

	if len(values) >= 7 {
		d.emitMakerNotesTag("FocusMode", int(values[6]))
	}
	if len(values) >= 16 {
		if iso := canonCameraISO(values[15]); iso != nil {
			d.emitMakerNotesTag("CameraISO", iso)
		}
	}
	if len(values) >= 22 {
		d.emitMakerNotesTag("LensType", int(values[21]))
	}
	if len(values) >= 25 {
		s.focalUnits = values[24]
	}
	if len(values) >= 34 {
		d.emitMakerNotesTag("ImageStabilization", int(values[33]))
	}
}

func (s *canonMakerNoteState) decodeFocalLength(d *videoDecoderMP4, value any) {
	values := coerceUint16Slice(value)
	if len(values) < 2 || s.focalUnits == 0 {
		return
	}

	d.emitMakerNotesTag("FocalLength", float64(values[1])/float64(s.focalUnits))
}

func (s *canonMakerNoteState) decodeShotInfo(d *videoDecoderMP4, value any) {
	values := coerceInt16Slice(value)
	if len(values) == 0 {
		return
	}

	if len(values) >= 6 {
		d.emitMakerNotesTag("ExposureCompensation", canonEV(values[5]))
	}
	if len(values) >= 7 {
		d.emitMakerNotesTag("WhiteBalance", int(values[6]))
	}
	if len(values) >= 21 && values[20] != 0 {
		d.emitMakerNotesTag("FNumber", math.Exp(canonEV(values[20])*math.Ln2/2))
	}
	if len(values) >= 22 && values[21] != 0 {
		d.emitMakerNotesTag("ExposureTime", math.Exp(-canonEV(values[21])*math.Ln2))
	}
}

func canonCameraISO(raw uint16) any {
	if raw == 0x7FFF {
		return nil
	}
	if raw&0x4000 != 0 {
		return int(raw & 0x3FFF)
	}

	switch raw {
	case 0:
		return "n/a"
	case 14:
		return "Auto High"
	case 15:
		return "Auto"
	case 16:
		return 50
	case 17:
		return 100
	case 18:
		return 200
	case 19:
		return 400
	case 20:
		return 800
	default:
		return "Unknown (" + itoa(int(raw)) + ")"
	}
}

func canonEV(raw int16) float64 {
	value := int(raw)
	sign := 1.0
	if value < 0 {
		value = -value
		sign = -1
	}

	frac := value & 0x1F
	value -= frac
	switch frac {
	case 0x0C:
		frac = 0x20 / 3
	case 0x14:
		frac = 0x40 / 3
	}

	return sign * float64(value+frac) / 32.0
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
