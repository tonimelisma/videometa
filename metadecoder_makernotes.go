package videometa

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
)

type makerNoteContext struct {
	byteOrder binary.ByteOrder
	make      string
	model     string
}

type makerNoteDecoder struct {
	*streamReader
	baseOffset int64
	seenIFDs   map[int64]bool
}

func newMakerNoteDecoder(data []byte, byteOrder binary.ByteOrder, baseOffset int64) *makerNoteDecoder {
	sr := newStreamReader(bytes.NewReader(data))
	sr.byteOrder = byteOrder
	return &makerNoteDecoder{
		streamReader: sr,
		baseOffset:   baseOffset,
		seenIFDs:     make(map[int64]bool),
	}
}

func (d *videoDecoderMP4) decodeMakerNotes(data []byte, ctx makerNoteContext) {
	if len(data) == 0 {
		return
	}

	makeUpper := strings.ToUpper(strings.TrimSpace(ctx.make))
	switch {
	case bytes.HasPrefix(data, []byte("Apple iOS\x00")):
		d.decodeAppleMakerNotes(data, ctx)
	case strings.HasPrefix(makeUpper, "CANON"):
		d.decodeCanonMakerNotes(data, ctx)
	case hasSonyMakerNoteHeader(data), strings.HasPrefix(makeUpper, "SONY"):
		d.decodeSonyMakerNotes(data, ctx)
	default:
		d.warnMakerNotes("unrecognized EXIF MakerNotes payload for make %q (%d bytes)", ctx.make, len(data))
	}
}

func hasSonyMakerNoteHeader(data []byte) bool {
	return bytes.HasPrefix(data, []byte("SONY DSC \x00")) ||
		bytes.HasPrefix(data, []byte("SONY CAM \x00")) ||
		bytes.HasPrefix(data, []byte("SONY MOBILE")) ||
		bytes.HasPrefix(data, []byte{0x00, 0x00, 'S', 'O', 'N', 'Y', ' ', 'P', 'I', 'C', 0x00}) ||
		bytes.HasPrefix(data, []byte("VHAB     \x00"))
}

func (md *makerNoteDecoder) walkIFD(startOffset int64, handle func(tagID, typ uint16, count uint32, value any)) {
	if md.seenIFDs[startOffset] {
		return
	}
	md.seenIFDs[startOffset] = true
	md.seek(startOffset)

	tagCount := md.read2()
	if tagCount > 2048 {
		return
	}

	for i := uint16(0); i < tagCount; i++ {
		tagID := md.read2()
		typ := md.read2()
		count := md.read4()
		valueFieldPos := md.pos()
		rawValue := md.read4()
		nextTagPos := md.pos()

		elemSize := exifTypeSize(typ)
		totalSize := int(count) * elemSize
		if totalSize <= 0 {
			continue
		}

		var value any
		if totalSize <= 4 {
			md.seek(valueFieldPos)
			value = md.readValue(typ, count)
			md.seek(nextTagPos)
		} else {
			target := md.baseOffset + int64(rawValue)
			if rawValue > uint32(math.MaxInt64-md.baseOffset) || target < 0 {
				md.seek(nextTagPos)
				continue
			}
			md.seek(target)
			value = md.readValue(typ, count)
			md.seek(nextTagPos)
		}

		if value != nil {
			handle(tagID, typ, count, value)
		}
	}

	nextIFDOffset := md.read4()
	if nextIFDOffset == 0 {
		return
	}

	target := md.baseOffset + int64(nextIFDOffset)
	if nextIFDOffset > uint32(math.MaxInt64-md.baseOffset) || target < 0 {
		return
	}
	md.walkIFD(target, handle)
}

func (md *makerNoteDecoder) readValue(typ uint16, count uint32) any {
	if count > 10000 {
		return nil
	}

	switch typ {
	case exifTypeByte:
		if count == 1 {
			return md.read1()
		}
		return md.readBytes(int(count))
	case exifTypeASCII:
		return printableString(string(trimNulls(md.readBytes(int(count)))))
	case exifTypeShort:
		if count == 1 {
			return md.read2()
		}
		values := make([]uint16, count)
		for i := range values {
			values[i] = md.read2()
		}
		return values
	case exifTypeLong:
		if count == 1 {
			return md.read4()
		}
		values := make([]uint32, count)
		for i := range values {
			values[i] = md.read4()
		}
		return values
	case exifTypeRational:
		if count == 1 {
			num := md.read4()
			den := md.read4()
			if den == 0 {
				return nil
			}
			return float64(num) / float64(den)
		}
		values := make([]float64, 0, count)
		for i := uint32(0); i < count; i++ {
			num := md.read4()
			den := md.read4()
			if den == 0 {
				values = append(values, 0)
				continue
			}
			values = append(values, float64(num)/float64(den))
		}
		return values
	case exifTypeUndef:
		return md.readBytes(int(count))
	case exifTypeSShort:
		if count == 1 {
			return int16(md.read2())
		}
		values := make([]int16, count)
		for i := range values {
			values[i] = int16(md.read2())
		}
		return values
	case exifTypeSLong:
		if count == 1 {
			return int32(md.read4())
		}
		values := make([]int32, count)
		for i := range values {
			values[i] = int32(md.read4())
		}
		return values
	case exifTypeSRational:
		if count == 1 {
			num := int32(md.read4())
			den := int32(md.read4())
			if den == 0 {
				return nil
			}
			return float64(num) / float64(den)
		}
		values := make([]float64, 0, count)
		for i := uint32(0); i < count; i++ {
			num := int32(md.read4())
			den := int32(md.read4())
			if den == 0 {
				values = append(values, 0)
				continue
			}
			values = append(values, float64(num)/float64(den))
		}
		return values
	default:
		return nil
	}
}

func coerceUint16Slice(value any) []uint16 {
	switch v := value.(type) {
	case uint16:
		return []uint16{v}
	case []uint16:
		return v
	case int16:
		return []uint16{uint16(v)}
	case []int16:
		values := make([]uint16, len(v))
		for i := range v {
			values[i] = uint16(v[i])
		}
		return values
	default:
		return nil
	}
}

func coerceInt16Slice(value any) []int16 {
	switch v := value.(type) {
	case int16:
		return []int16{v}
	case []int16:
		return v
	case uint16:
		return []int16{int16(v)}
	case []uint16:
		values := make([]int16, len(v))
		for i := range v {
			values[i] = int16(v[i])
		}
		return values
	default:
		return nil
	}
}

func (d *videoDecoderMP4) warnMakerNotes(format string, args ...any) {
	if d.opts.Warnf != nil {
		d.opts.Warnf("decode makernotes: "+format, args...)
	}
}
