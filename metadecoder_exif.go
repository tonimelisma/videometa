package videometa

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strings"
)

// EXIF UUID used to locate EXIF data in ISOBMFF containers.
var exifUUID = [16]byte{
	0x21, 0xC2, 0x6C, 0x08, 0x67, 0x45, 0x11, 0xDA,
	0xAB, 0x07, 0xD0, 0x17, 0x05, 0x93, 0x9E, 0x53,
}

// EXIF data types and their sizes.
const (
	exifTypeByte      uint16 = 1
	exifTypeASCII     uint16 = 2
	exifTypeShort     uint16 = 3
	exifTypeLong      uint16 = 4
	exifTypeRational  uint16 = 5
	exifTypeSByte     uint16 = 6
	exifTypeUndef     uint16 = 7
	exifTypeSShort    uint16 = 8
	exifTypeSLong     uint16 = 9
	exifTypeSRational uint16 = 10
	exifTypeFloat     uint16 = 11
	exifTypeDouble    uint16 = 12
)

// exifTypeSize returns the byte size of a single EXIF type element.
func exifTypeSize(typ uint16) int {
	switch typ {
	case exifTypeByte, exifTypeASCII, exifTypeSByte, exifTypeUndef:
		return 1
	case exifTypeShort, exifTypeSShort:
		return 2
	case exifTypeLong, exifTypeSLong, exifTypeFloat:
		return 4
	case exifTypeRational, exifTypeSRational, exifTypeDouble:
		return 8
	default:
		return 0
	}
}

// metaDecoderEXIF decodes EXIF IFD structures from a byte buffer.
type metaDecoderEXIF struct {
	*streamReader
	opts     Options
	seenIFDs map[int64]bool // Prevent infinite IFD loops.

	// GPS tag accumulation for coordinate conversion.
	gpsLatRef string
	gpsLonRef string
	gpsLat    [3]float64
	gpsLon    [3]float64
	hasGPSLat bool
	hasGPSLon bool
	gpsAltRef uint8
	gpsAlt    float64
	hasGPSAlt bool
}

// decodeEXIF parses EXIF data from the given ReadSeeker.
// The reader should be positioned at the start of the TIFF header (II or MM).
func (d *videoDecoderMP4) decodeEXIF(r io.ReadSeeker) {
	sr := &streamReader{
		r:         r,
		rs:        r,
		byteOrder: binary.BigEndian,
		canSeek:   true,
	}

	ed := &metaDecoderEXIF{
		streamReader: sr,
		opts:         d.opts,
		seenIFDs:     make(map[int64]bool),
	}

	// Recover panics from EXIF decoding — partial failure is OK.
	defer func() {
		if rec := recover(); rec != nil {
			if rec != errStop {
				// Unexpected panic — warn but don't crash.
				if d.opts.Warnf != nil {
					d.opts.Warnf("decode exif: unexpected panic: %v", rec)
				}
			}
			// errStop means we hit EOF in the EXIF data — that's fine.
		}
	}()

	ed.decode(d)
}

func (ed *metaDecoderEXIF) decode(d *videoDecoderMP4) {
	// Read byte order marker.
	marker := ed.read2()
	switch marker {
	case 0x4949: // "II" — little-endian
		ed.byteOrder = binary.LittleEndian
	case 0x4D4D: // "MM" — big-endian
		ed.byteOrder = binary.BigEndian
	default:
		if d.opts.Warnf != nil {
			d.opts.Warnf("decode exif: invalid byte order marker: 0x%04X", marker)
		}
		return
	}

	// Verify magic number 0x002A.
	magic := ed.read2()
	if magic != 0x002A {
		if d.opts.Warnf != nil {
			d.opts.Warnf("decode exif: invalid TIFF magic: 0x%04X", magic)
		}
		return
	}

	// Read IFD0 offset and seek to it.
	ifd0Offset := ed.read4()
	ed.seek(int64(ifd0Offset))

	ed.decodeTags(d, "IFD0", exifFields)

	// After IFD0, there may be an IFD1 offset for thumbnails — skip it.
}

func (ed *metaDecoderEXIF) decodeTags(d *videoDecoderMP4, namespace string, fields map[uint16]string) {
	offset := ed.pos()
	if ed.seenIFDs[offset] {
		return // Prevent infinite loops.
	}
	ed.seenIFDs[offset] = true

	tagCount := ed.read2()
	if tagCount > 1000 {
		return // Sanity check.
	}

	for i := uint16(0); i < tagCount; i++ {
		ed.decodeTag(d, namespace, fields)
	}

	// Emit accumulated GPS coordinates after processing all tags.
	if namespace == "GPSInfoIFD" {
		ed.emitGPSCoordinates(d)
	}
}

func (ed *metaDecoderEXIF) decodeTag(d *videoDecoderMP4, namespace string, fields map[uint16]string) {
	tagID := ed.read2()
	typ := ed.read2()
	count := ed.read4()
	// Save the position of the 4-byte value/offset field.
	valueFieldPos := ed.pos()
	rawValue := ed.read4() // 4-byte value or offset
	// After reading the tag entry, we're positioned at the next tag.
	// Save this position to restore after any seeking.
	nextTagPos := ed.pos()

	elemSize := exifTypeSize(typ)
	totalSize := int(count) * elemSize

	// Check for IFD pointers (sub-IFDs).
	if subIFDName, ok := exifIFDPointers[tagID]; ok {
		ed.seek(int64(rawValue))
		subFields := exifFields
		switch subIFDName {
		case "GPSInfoIFD":
			subFields = exifFieldsGPS
		case "InteropIFD":
			subFields = exifInteropFields
		}
		ed.decodeTags(d, subIFDName, subFields)
		ed.seek(nextTagPos)
		return
	}

	tagName := fields[tagID]
	if tagName == "" {
		return // Unknown tag — skip.
	}

	// Read the value. If totalSize > 4, rawValue is an offset to the data.
	var value any
	if totalSize <= 4 && totalSize > 0 {
		// Value is stored inline in the 4-byte field.
		ed.seek(valueFieldPos)
		value = ed.readValue(typ, count)
		ed.seek(nextTagPos)
	} else if totalSize > 4 {
		// rawValue is an offset to the data.
		ed.seek(int64(rawValue))
		value = ed.readValue(typ, count)
		ed.seek(nextTagPos)
	}

	if value == nil {
		return
	}

	// Accumulate GPS fields for later coordinate conversion.
	if namespace == "GPSInfoIFD" {
		ed.accumulateGPS(tagID, value)
	}

	// Skip raw GPS coordinate arrays — we emit converted decimal degrees instead.
	if namespace == "GPSInfoIFD" && (tagID == 0x0002 || tagID == 0x0004) {
		return
	}

	// Route IPTC data embedded in EXIF ApplicationNotes tag (0x83BB).
	if tagID == 0x83BB && typ == exifTypeUndef {
		if data, ok := value.([]byte); ok && d.opts.Sources.Has(IPTC) {
			d.decodeIPTC(bytes.NewReader(data))
		}
		return
	}

	// Route MakerNotes (0x927C) to manufacturer-specific decoder.
	if tagID == 0x927C {
		if data, ok := value.([]byte); ok {
			d.decodeMakerNotes(data)
		}
		return
	}

	d.emitEXIFTag(tagName, namespace, value)
}

func (ed *metaDecoderEXIF) readValue(typ uint16, count uint32) any {
	if count > 10000 {
		return nil // Sanity check.
	}

	switch typ {
	case exifTypeByte:
		if count == 1 {
			return ed.read1()
		}
		return ed.readBytes(int(count))
	case exifTypeASCII:
		b := ed.readBytes(int(count))
		return printableString(string(trimNulls(b)))
	case exifTypeShort:
		if count == 1 {
			return ed.read2()
		}
		vals := make([]uint16, count)
		for i := range vals {
			vals[i] = ed.read2()
		}
		return vals
	case exifTypeLong:
		if count == 1 {
			return ed.read4()
		}
		vals := make([]uint32, count)
		for i := range vals {
			vals[i] = ed.read4()
		}
		return vals
	case exifTypeRational:
		if count == 1 {
			num := ed.read4()
			den := ed.read4()
			r, err := NewRat[uint32](num, den)
			if err != nil {
				return float64(num)
			}
			return r
		}
		vals := make([]any, count)
		for i := range vals {
			num := ed.read4()
			den := ed.read4()
			r, err := NewRat[uint32](num, den)
			if err != nil {
				vals[i] = float64(num)
			} else {
				vals[i] = r
			}
		}
		return vals
	case exifTypeSByte:
		if count == 1 {
			return int8(ed.read1())
		}
		return ed.readBytes(int(count))
	case exifTypeUndef:
		return ed.readBytes(int(count))
	case exifTypeSShort:
		if count == 1 {
			return int16(ed.read2())
		}
		vals := make([]int16, count)
		for i := range vals {
			vals[i] = int16(ed.read2())
		}
		return vals
	case exifTypeSLong:
		if count == 1 {
			return ed.read4s()
		}
		vals := make([]int32, count)
		for i := range vals {
			vals[i] = ed.read4s()
		}
		return vals
	case exifTypeSRational:
		if count == 1 {
			num := ed.read4s()
			den := ed.read4s()
			r, err := NewRat[int32](num, den)
			if err != nil {
				return float64(num)
			}
			return r
		}
		vals := make([]any, count)
		for i := range vals {
			num := ed.read4s()
			den := ed.read4s()
			r, err := NewRat[int32](num, den)
			if err != nil {
				vals[i] = float64(num)
			} else {
				vals[i] = r
			}
		}
		return vals
	case exifTypeFloat:
		if count == 1 {
			bits := ed.read4()
			return float64(math.Float32frombits(bits))
		}
		vals := make([]float64, count)
		for i := range vals {
			bits := ed.read4()
			vals[i] = float64(math.Float32frombits(bits))
		}
		return vals
	case exifTypeDouble:
		if count == 1 {
			bits := ed.read8()
			return math.Float64frombits(bits)
		}
		vals := make([]float64, count)
		for i := range vals {
			bits := ed.read8()
			vals[i] = math.Float64frombits(bits)
		}
		return vals
	default:
		return nil
	}
}

// accumulateGPS stores GPS tag values for later coordinate conversion.
func (ed *metaDecoderEXIF) accumulateGPS(tagID uint16, value any) {
	switch tagID {
	case 0x0001: // GPSLatitudeRef
		ed.gpsLatRef = toString(value)
	case 0x0002: // GPSLatitude (3 rationals)
		ed.hasGPSLat = true
		ed.gpsLat = rationalsToFloats(value)
	case 0x0003: // GPSLongitudeRef
		ed.gpsLonRef = toString(value)
	case 0x0004: // GPSLongitude (3 rationals)
		ed.hasGPSLon = true
		ed.gpsLon = rationalsToFloats(value)
	case 0x0005: // GPSAltitudeRef
		if v, ok := value.(uint8); ok {
			ed.gpsAltRef = v
		}
	case 0x0006: // GPSAltitude
		ed.hasGPSAlt = true
		if r, ok := value.(Rat[uint32]); ok {
			ed.gpsAlt = r.Float64()
		} else if f, ok := toFloat64(value); ok {
			ed.gpsAlt = f
		}
	}
}

// emitGPSCoordinates converts accumulated GPS DMS values to decimal degrees.
func (ed *metaDecoderEXIF) emitGPSCoordinates(d *videoDecoderMP4) {
	if ed.hasGPSLat {
		lat := convertDegreesToDecimal(ed.gpsLat[0], ed.gpsLat[1], ed.gpsLat[2])
		if strings.EqualFold(ed.gpsLatRef, "S") {
			lat = -lat
		}
		d.emitEXIFTag("GPSLatitude", "GPSInfoIFD", lat)
	}
	if ed.hasGPSLon {
		lon := convertDegreesToDecimal(ed.gpsLon[0], ed.gpsLon[1], ed.gpsLon[2])
		if strings.EqualFold(ed.gpsLonRef, "W") {
			lon = -lon
		}
		d.emitEXIFTag("GPSLongitude", "GPSInfoIFD", lon)
	}
	if ed.hasGPSAlt {
		alt := ed.gpsAlt
		if ed.gpsAltRef == 1 {
			alt = -alt
		}
		d.emitEXIFTag("GPSAltitude", "GPSInfoIFD", alt)
	}
}

// rationalsToFloats extracts 3 float64 values from a slice of rationals.
func rationalsToFloats(v any) [3]float64 {
	var result [3]float64
	switch vals := v.(type) {
	case []any:
		for i := 0; i < 3 && i < len(vals); i++ {
			if r, ok := vals[i].(Rat[uint32]); ok {
				result[i] = r.Float64()
			} else if f, ok := toFloat64(vals[i]); ok {
				result[i] = f
			}
		}
	}
	return result
}

// emitEXIFTag sends an EXIF source tag via the centralized emitTag.
func (d *videoDecoderMP4) emitEXIFTag(name, namespace string, value any) {
	d.emitTag(TagInfo{
		Source:    EXIF,
		Tag:       name,
		Namespace: namespace,
		Value:     value,
	})
}
