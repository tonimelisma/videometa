package videometa

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// QuickTime epoch: 1904-01-01T00:00:00Z. Offset from Unix epoch.
const quickTimeEpochOffset = 2082844800

// Known ftyp brands for MP4/MOV.
var (
	mp4Brands = map[string]bool{
		"isom": true, "iso2": true, "iso3": true, "iso4": true,
		"iso5": true, "iso6": true, "mp41": true, "mp42": true,
		"avc1": true, "hvc1": true, "hev1": true, "av01": true,
		"M4V ": true, "M4A ": true, "f4v ": true, "mmp4": true,
		"MSNV": true, "dash": true, "3gp4": true, "3gp5": true,
		"3gp6": true, "3gp7": true,
	}
	movBrands = map[string]bool{
		"qt  ": true,
	}
)

// videoDecoderMP4 handles MP4/MOV container parsing and metadata routing.
type videoDecoderMP4 struct {
	*baseDecoder
	isMOV bool // True if ftyp brand indicates QuickTime MOV.
}

func (d *videoDecoderMP4) decode() error {
	// Iterate top-level boxes.
	for {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "ftyp":
			if err := d.decodeFtyp(startPos, boxSize); err != nil {
				return err
			}
		case "moov":
			if err := d.decodeMoov(startPos, boxSize); err != nil {
				return err
			}
		case "uuid":
			d.decodeUUID(startPos, boxSize)
		case "moof":
			return newInvalidFormatErrorf("fragmented MP4 (moof box) not supported")
		case "mdat", "free", "skip", "wide":
			// Skip media data and padding boxes.
		default:
			// Skip unknown top-level boxes silently.
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
	return nil
}

// readBoxHeader reads an ISOBMFF box header and returns (size, type, isEOF).
// Size includes the header itself. Returns isEOF=true on clean EOF.
func (d *videoDecoderMP4) readBoxHeader() (totalSize uint64, boxType fourCC, isEOF bool) {
	// Read 4-byte size.
	var sizeBuf [4]byte
	n, err := d.r.Read(sizeBuf[:])
	if n == 0 && err != nil {
		return 0, fourCC{}, true
	}
	d.readerOffset += int64(n)
	if n < 4 {
		// Partial read — try to get the rest.
		remaining := sizeBuf[:]
		d.readFull(remaining[n:])
	}

	size := d.byteOrder.Uint32(sizeBuf[:])
	boxType = d.readFourCC()
	totalSize = uint64(size)

	if size == 1 {
		// 64-bit extended size.
		totalSize = d.read8()
	} else if size == 0 {
		// Box extends to EOF. Use a large sentinel value.
		// This is only valid for the last box (typically mdat).
		totalSize = 1<<63 - 1
	}

	return totalSize, boxType, false
}

// seekToBoxEnd seeks past the end of a box.
func (d *videoDecoderMP4) seekToBoxEnd(startPos int64, boxSize uint64) {
	endPos := startPos + int64(boxSize)
	currentPos := d.pos()
	if currentPos < endPos {
		d.skip(endPos - currentPos)
	}
}

// decodeFtyp validates the ftyp box and sets isMOV flag.
func (d *videoDecoderMP4) decodeFtyp(startPos int64, boxSize uint64) error {
	majorBrand := d.readFourCC()
	_ = d.read4() // minor version

	brandStr := majorBrand.String()

	// Check major brand.
	if movBrands[brandStr] {
		d.isMOV = true
		return nil
	}
	if mp4Brands[brandStr] {
		return nil
	}

	// Check compatible brands.
	endPos := startPos + int64(boxSize)
	for d.pos() < endPos {
		compat := d.readFourCC()
		compatStr := compat.String()
		if movBrands[compatStr] {
			d.isMOV = true
			return nil
		}
		if mp4Brands[compatStr] {
			return nil
		}
	}

	return newInvalidFormatErrorf("unrecognized ftyp brand: %q", brandStr)
}

// decodeMoov iterates the moov container's child boxes.
func (d *videoDecoderMP4) decodeMoov(moovStart int64, moovSize uint64) error {
	moovEnd := moovStart + int64(moovSize)
	for d.pos() < moovEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "mvhd":
			d.decodeMvhd()
		case "trak":
			d.decodeTrak(startPos, boxSize)
		case "udta":
			d.decodeUdta(startPos, boxSize)
		case "meta":
			d.decodeMeta(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
	return nil
}

// decodeMvhd parses the movie header box (mvhd).
func (d *videoDecoderMP4) decodeMvhd() {
	version := d.read1()
	_ = d.readBytes(3) // flags

	var creationTime, modificationTime uint64
	var timescale uint32
	var duration uint64

	if version == 1 {
		creationTime = d.read8()
		modificationTime = d.read8()
		timescale = d.read4()
		duration = d.read8()
	} else {
		creationTime = uint64(d.read4())
		modificationTime = uint64(d.read4())
		timescale = d.read4()
		duration = uint64(d.read4())
	}

	// Convert to time.Time and duration.
	if timescale > 0 {
		d.result.VideoConfig.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("CreateDate", quickTimeToTime(creationTime))
		d.emitQuickTimeTag("ModifyDate", quickTimeToTime(modificationTime))
		d.emitQuickTimeTag("TimeScale", timescale)
		d.emitQuickTimeTag("Duration", durationSeconds(duration, timescale))

		// Read remaining mvhd fields.
		preferredRate := d.read4() // 16.16 fixed point
		preferredVolume := d.read2()
		_ = d.readBytes(10) // reserved

		d.emitQuickTimeTag("PreferredRate", fixedPoint1616ToInt(preferredRate))
		d.emitQuickTimeTag("PreferredVolume", fixedPoint88ToFloat(preferredVolume))

		// Matrix (9 x 4 bytes = 36 bytes).
		_ = d.readBytes(36)

		// Preview, poster, selection, current time.
		d.emitQuickTimeTag("PreviewTime", d.read4())
		d.emitQuickTimeTag("PreviewDuration", d.read4())
		d.emitQuickTimeTag("PosterTime", d.read4())
		d.emitQuickTimeTag("SelectionTime", d.read4())
		d.emitQuickTimeTag("SelectionDuration", d.read4())
		d.emitQuickTimeTag("CurrentTime", d.read4())
		d.emitQuickTimeTag("NextTrackID", d.read4())
	}
}

// decodeTrak iterates a track box's children.
func (d *videoDecoderMP4) decodeTrak(trakStart int64, trakSize uint64) {
	trakEnd := trakStart + int64(trakSize)
	for d.pos() < trakEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "tkhd":
			d.decodeTkhd()
		case "mdia":
			d.decodeMdia(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeTkhd parses the track header box (tkhd).
func (d *videoDecoderMP4) decodeTkhd() {
	version := d.read1()
	_ = d.readBytes(3) // flags

	var creationTime, modificationTime uint64
	var trackID uint32
	var duration uint64

	if version == 1 {
		creationTime = d.read8()
		modificationTime = d.read8()
		trackID = d.read4()
		_ = d.read4() // reserved
		duration = d.read8()
	} else {
		creationTime = uint64(d.read4())
		modificationTime = uint64(d.read4())
		trackID = d.read4()
		_ = d.read4() // reserved
		duration = uint64(d.read4())
	}

	_ = d.readBytes(8) // reserved

	layer := d.read2()
	alternateGroup := d.read2()
	volume := d.read2()
	_ = d.read2() // reserved
	_ = alternateGroup

	// Read transformation matrix (9 x int32 = 36 bytes).
	var matrix [9]int32
	for i := range matrix {
		matrix[i] = d.read4s()
	}

	// Width and height are 16.16 fixed point.
	widthFixed := d.read4()
	heightFixed := d.read4()
	width := int(widthFixed >> 16)
	height := int(heightFixed >> 16)

	// Extract rotation from matrix.
	rotation := matrixToRotation(matrix)

	// Only use dimensions from the first video track.
	if d.result.VideoConfig.Width == 0 && width > 0 {
		d.result.VideoConfig.Width = width
		d.result.VideoConfig.Height = height
		d.result.VideoConfig.Rotation = rotation
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("TrackHeaderVersion", version)
		d.emitQuickTimeTag("TrackCreateDate", quickTimeToTime(creationTime))
		d.emitQuickTimeTag("TrackModifyDate", quickTimeToTime(modificationTime))
		d.emitQuickTimeTag("TrackID", trackID)
		d.emitQuickTimeTag("TrackDuration", durationSeconds(duration, 1000)) // tkhd duration is in movie timescale
		d.emitQuickTimeTag("TrackLayer", layer)
		d.emitQuickTimeTag("TrackVolume", fixedPoint88ToFloat(volume))
		d.emitQuickTimeTag("ImageWidth", width)
		d.emitQuickTimeTag("ImageHeight", height)
		d.emitQuickTimeTag("MatrixStructure", formatMatrix(matrix))
	}
}

// decodeMdia iterates the media container box.
func (d *videoDecoderMP4) decodeMdia(mdiaStart int64, mdiaSize uint64) {
	mdiaEnd := mdiaStart + int64(mdiaSize)
	for d.pos() < mdiaEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "mdhd":
			d.decodeMdhd()
		case "hdlr":
			d.decodeHdlr()
		case "minf":
			d.decodeMinf(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeMdhd parses the media header box.
func (d *videoDecoderMP4) decodeMdhd() {
	version := d.read1()
	_ = d.readBytes(3) // flags

	var creationTime, modificationTime uint64
	var timescale uint32
	var duration uint64

	if version == 1 {
		creationTime = d.read8()
		modificationTime = d.read8()
		timescale = d.read4()
		duration = d.read8()
	} else {
		creationTime = uint64(d.read4())
		modificationTime = uint64(d.read4())
		timescale = d.read4()
		duration = uint64(d.read4())
	}

	// Language code: packed ISO-639-2/T.
	langCode := d.read2()
	lang := decodeISO639(langCode)

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("MediaHeaderVersion", version)
		d.emitQuickTimeTag("MediaCreateDate", quickTimeToTime(creationTime))
		d.emitQuickTimeTag("MediaModifyDate", quickTimeToTime(modificationTime))
		d.emitQuickTimeTag("MediaTimeScale", timescale)
		d.emitQuickTimeTag("MediaDuration", durationSeconds(duration, timescale))
		d.emitQuickTimeTag("MediaLanguageCode", lang)
	}
}

// decodeHdlr parses the handler reference box.
func (d *videoDecoderMP4) decodeHdlr() {
	_ = d.readBytes(4) // version + flags
	_ = d.read4()      // pre_defined

	handlerType := d.readFourCC()

	// Read manufacturer/vendor (12 bytes = 3 x uint32 reserved).
	_ = d.readBytes(12)

	// The rest is a null-terminated name string, but we don't know its length.
	// We'll just skip it (the parent box size handles positioning).

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("HandlerType", handlerType.String())
	}
}

// decodeMinf iterates the media information box.
func (d *videoDecoderMP4) decodeMinf(minfStart int64, minfSize uint64) {
	minfEnd := minfStart + int64(minfSize)
	for d.pos() < minfEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		if boxType.String() == "stbl" {
			d.decodeStbl(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeStbl iterates the sample table box.
func (d *videoDecoderMP4) decodeStbl(stblStart int64, stblSize uint64) {
	stblEnd := stblStart + int64(stblSize)
	for d.pos() < stblEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		if boxType.String() == "stsd" {
			d.decodeStsd()
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeStsd parses the sample description box to extract codec info.
func (d *videoDecoderMP4) decodeStsd() {
	_ = d.readBytes(4) // version + flags
	entryCount := d.read4()

	if entryCount == 0 {
		return
	}

	// Read first sample entry.
	_ = d.read4()             // entry size
	codec := d.readFourCC()   // codec fourCC (e.g., "avc1", "hvc1")
	_ = d.readBytes(6)        // reserved
	_ = d.read2()             // data reference index

	// Set codec in VideoConfig (first track only).
	if d.result.VideoConfig.Codec == "" {
		d.result.VideoConfig.Codec = codec.String()
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("CompressorID", codec.String())
	}
}

// decodeUdta iterates the user data box.
func (d *videoDecoderMP4) decodeUdta(udtaStart int64, udtaSize uint64) {
	udtaEnd := udtaStart + int64(udtaSize)
	for d.pos() < udtaEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		if boxType.String() == "meta" {
			d.decodeMeta(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeMeta parses the metadata container box (FullBox: has version+flags).
func (d *videoDecoderMP4) decodeMeta(metaStart int64, metaSize uint64) {
	// The meta box is supposed to be a FullBox (4 bytes version+flags), but some
	// encoders omit the FullBox header. Detect by peeking at the first 8 bytes:
	// if they form a valid box header (reasonable size + ASCII fourcc), skip the
	// FullBox header; otherwise consume 4 bytes as version+flags.
	if d.canSeek {
		peekPos := d.pos()
		firstFour := d.read4()
		nextFour := d.readFourCC()
		d.seek(peekPos)

		isValidBox := firstFour > 0 && firstFour < uint32(metaSize) && isASCIIFourCC(nextFour)
		if !isValidBox {
			_ = d.readBytes(4) // Standard FullBox — consume version+flags.
		}
	} else {
		// Non-seekable: assume FullBox (most common case).
		_ = d.readBytes(4)
	}

	metaEnd := metaStart + int64(metaSize)

	// Track the handler type and keys for mdta-style metadata.
	var handlerType string
	var keysTable []string

	for d.pos() < metaEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "hdlr":
			handlerType = d.decodeMetaHdlrReturn(startPos, boxSize)
		case "keys":
			keysTable = d.decodeKeysBox(startPos, boxSize)
		case "ilst":
			if d.opts.Sources.Has(QUICKTIME) {
				if handlerType == "mdta" && len(keysTable) > 0 {
					d.decodeIlstMdta(startPos, boxSize, keysTable)
				} else {
					d.decodeIlst(startPos, boxSize)
				}
			}
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeMetaHdlrReturn parses the handler box inside meta and returns the handler type.
func (d *videoDecoderMP4) decodeMetaHdlrReturn(hdlrStart int64, hdlrSize uint64) string {
	_ = d.readBytes(4) // version + flags
	_ = d.read4()      // pre_defined
	handlerType := d.readFourCC()

	// Read vendor ID (first 4 bytes of reserved).
	vendorID := d.readFourCC()
	_ = d.readBytes(8) // remaining reserved

	// Read handler description (name) — rest of box is a null-terminated string.
	endPos := hdlrStart + int64(hdlrSize)
	nameLen := endPos - d.pos()
	var description string
	if nameLen > 0 && nameLen < 256 {
		nameBytes := d.readBytes(int(nameLen))
		description = printableString(string(trimNulls(nameBytes)))
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("HandlerType", handlerType.String())
		d.emitQuickTimeTag("HandlerVendorID", vendorID.String())
		if description != "" {
			d.emitQuickTimeTag("HandlerDescription", description)
		}
	}

	return handlerType.String()
}

// decodeKeysBox parses the keys box (mdta-style metadata key definitions).
// Returns a 1-indexed slice of key names (index 0 is unused).
func (d *videoDecoderMP4) decodeKeysBox(keysStart int64, keysSize uint64) []string {
	_ = d.readBytes(4) // version + flags
	entryCount := d.read4()

	if entryCount > 10000 {
		return nil // Sanity check.
	}

	// Index 0 is unused — keys are 1-indexed.
	keys := make([]string, entryCount+1)
	for i := uint32(1); i <= entryCount; i++ {
		keySize := d.read4()     // Size including this 4 bytes + namespace(4) + key string.
		namespace := d.readFourCC() // e.g., "mdta"
		_ = namespace
		nameLen := int(keySize) - 8
		if nameLen <= 0 || nameLen > 1024 {
			break
		}
		keys[i] = string(d.readBytes(nameLen))
	}
	return keys
}

// decodeIlstMdta parses an ilst box where entries reference a keys table by 1-based index.
func (d *videoDecoderMP4) decodeIlstMdta(ilstStart int64, ilstSize uint64, keys []string) {
	ilstEnd := ilstStart + int64(ilstSize)
	for d.pos() < ilstEnd {
		atomStart := d.pos()
		atomSize, atomType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}
		atomEnd := atomStart + int64(atomSize)

		// The atom type is a big-endian uint32 index into the keys table.
		keyIndex := uint32(atomType[0])<<24 | uint32(atomType[1])<<16 | uint32(atomType[2])<<8 | uint32(atomType[3])
		if keyIndex > 0 && int(keyIndex) < len(keys) {
			keyName := keys[keyIndex]
			tagName := freeformToTagName("com.apple.quicktime", mdtaKeyToShortName(keyName))
			if tagName != "" {
				d.decodeIlstAtomData(atomStart, atomSize, tagName)
			}
		}

		if d.pos() < atomEnd {
			d.skip(atomEnd - d.pos())
		}
	}
}

// mdtaKeyToShortName extracts the short key name from a fully qualified mdta key.
// E.g., "com.apple.quicktime.creationdate" → "creationdate"
func mdtaKeyToShortName(key string) string {
	const prefix = "com.apple.quicktime."
	if len(key) > len(prefix) && key[:len(prefix)] == prefix {
		return key[len(prefix):]
	}
	return key
}

// decodeUUID handles UUID extended-type boxes. XMP and EXIF data in MP4
// can be stored in UUID boxes identified by specific 16-byte GUIDs.
func (d *videoDecoderMP4) decodeUUID(startPos int64, boxSize uint64) {
	// The UUID is the first 16 bytes after the standard 8-byte box header.
	var uuid [16]byte
	d.readBytesInto(uuid[:])

	// Remaining data length = box size - 8 (header) - 16 (UUID).
	dataLen := int64(boxSize) - 24
	if dataLen <= 0 {
		return
	}

	switch uuid {
	case xmpUUID:
		if d.opts.Sources.Has(XMP) {
			rc := d.bufferedReader(int(dataLen))
			defer rc.Close()
			d.decodeXMP(rc)
		}
	case exifUUID:
		if d.opts.Sources.Has(EXIF) {
			// EXIF in MP4 UUID box has a 4-byte header offset prefix.
			headerOffset := d.read4()
			rc := d.bufferedReader(int(dataLen) - 4)
			defer rc.Close()
			// Skip the header offset bytes within the buffered reader.
			if headerOffset > 0 {
				buf := make([]byte, headerOffset)
				io.ReadFull(rc, buf)
			}
			d.decodeEXIF(rc)
		}
	}
}

// emitQuickTimeTag sends a QuickTime source tag to the callback.
func (d *videoDecoderMP4) emitQuickTimeTag(name string, value any) {
	if d.opts.HandleTag == nil {
		return
	}
	ti := TagInfo{
		Source:    QUICKTIME,
		Tag:       name,
		Namespace: "QuickTime",
		Value:     value,
	}
	if d.opts.ShouldHandleTag != nil && !d.opts.ShouldHandleTag(ti) {
		return
	}
	if err := d.opts.HandleTag(ti); err != nil {
		panic(err) // ErrStopWalking recovered at Decode() boundary.
	}
}

// quickTimeToTime converts a QuickTime timestamp (seconds since 1904-01-01) to time.Time.
func quickTimeToTime(qtTime uint64) time.Time {
	if qtTime == 0 {
		return time.Time{}
	}
	unixSeconds := int64(qtTime) - quickTimeEpochOffset
	return time.Unix(unixSeconds, 0).UTC()
}

// durationSeconds converts a duration in timescale units to a float64 seconds value.
func durationSeconds(duration uint64, timescale uint32) float64 {
	if timescale == 0 {
		return 0
	}
	return float64(duration) / float64(timescale)
}

// fixedPoint1616ToInt converts a 16.16 fixed-point value to an integer.
func fixedPoint1616ToInt(v uint32) int {
	return int(v >> 16)
}

// fixedPoint88ToFloat converts an 8.8 fixed-point value to float64.
func fixedPoint88ToFloat(v uint16) float64 {
	return float64(v) / 256.0
}

// matrixToRotation extracts rotation degrees from a transformation matrix.
// The matrix is [a b u c d v x y w] in 16.16 fixed point (u,v,w are 2.30).
func matrixToRotation(m [9]int32) int {
	a := m[0]
	b := m[1]
	c := m[3]
	d := m[4]

	// Standard rotation matrices (16.16 fixed point where 1.0 = 0x10000):
	switch {
	case a == 0x10000 && b == 0 && c == 0 && d == 0x10000:
		return 0 // Identity
	case a == 0 && b == 0x10000 && c == -0x10000 && d == 0:
		return 90
	case a == -0x10000 && b == 0 && c == 0 && d == -0x10000:
		return 180
	case a == 0 && b == -0x10000 && c == 0x10000 && d == 0:
		return 270
	default:
		return 0
	}
}

// formatMatrix formats a 3x3 matrix as a space-delimited string matching exiftool.
// Values are 16.16 fixed point, but the last row is 2.30 fixed point.
func formatMatrix(m [9]int32) string {
	return fmt.Sprintf("%d %d %d %d %d %d %d %d %d",
		fixedPoint1616ToInt(uint32(m[0])),
		fixedPoint1616ToInt(uint32(m[1])),
		fixedPoint1616ToInt(uint32(m[2])),
		fixedPoint1616ToInt(uint32(m[3])),
		fixedPoint1616ToInt(uint32(m[4])),
		fixedPoint1616ToInt(uint32(m[5])),
		fixedPoint1616ToInt(uint32(m[6])),
		fixedPoint1616ToInt(uint32(m[7])),
		// Last element is 2.30 fixed point: 0x40000000 = 1.0
		m[8]>>14, // Convert 2.30 to ~16.16 then shift
	)
}

// decodeISO639 decodes a packed ISO-639-2/T language code.
func decodeISO639(packed uint16) string {
	// Each character is 5 bits, offset by 0x60.
	c1 := byte((packed>>10)&0x1F) + 0x60
	c2 := byte((packed>>5)&0x1F) + 0x60
	c3 := byte(packed&0x1F) + 0x60

	s := string([]byte{c1, c2, c3})
	if s == "\x60\x60\x60" {
		return "und" // undetermined
	}
	return s
}

// readBoxHeaderFrom reads a box header, handling the "not enough data" case
// as EOF for sub-box iteration. Uses the provided byte order.
func (d *videoDecoderMP4) readBoxHeaderFrom(r *streamReader) (totalSize uint64, boxType fourCC, isEOF bool) {
	_ = r
	return d.readBoxHeader()
}

// isASCIIFourCC returns true if all 4 bytes are printable ASCII (0x20-0x7E).
func isASCIIFourCC(fcc fourCC) bool {
	for _, b := range fcc {
		if b < 0x20 || b > 0x7E {
			return false
		}
	}
	return true
}

// Ensure big-endian is always used for ISOBMFF.
func init() {
	_ = binary.BigEndian
}
