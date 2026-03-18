package videometa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// QuickTime epoch: 1904-01-01T00:00:00Z. Offset from Unix epoch.
const quickTimeEpochOffset = 2082844800

// UUID constants for extended-type boxes.
var (
	profUUID = [16]byte{
		0x50, 0x52, 0x4F, 0x46, 0x21, 0xD2, 0x4F, 0xCE,
		0xBB, 0x88, 0x69, 0x5C, 0xFA, 0xC9, 0xC7, 0x40,
	}
	usmtUUID = [16]byte{
		0x55, 0x53, 0x4D, 0x54, 0x21, 0xD2, 0x4F, 0xCE,
		0xBB, 0x88, 0x69, 0x5C, 0xFA, 0xC9, 0xC7, 0x40,
	}
)

// Known ftyp brands for MP4/MOV.
var (
	mp4Brands = map[string]bool{
		"isom": true, "iso2": true, "iso3": true, "iso4": true,
		"iso5": true, "iso6": true, "mp41": true, "mp42": true,
		"avc1": true, "hvc1": true, "hev1": true, "av01": true,
		"M4V ": true, "M4A ": true, "f4v ": true, "mmp4": true,
		"MSNV": true, "dash": true, "XAVC": true,
		"3gp4": true, "3gp5": true, "3gp6": true, "3gp7": true,
		"nras": true,
	}
	movBrands = map[string]bool{
		"qt  ": true,
	}
)

// videoDecoderMP4 handles MP4/MOV container parsing and metadata routing.
type videoDecoderMP4 struct {
	*baseDecoder
	isMOV              bool   // True if ftyp brand indicates QuickTime MOV.
	movieTimescale     uint32 // From mvhd, used for tkhd TrackDuration conversion.
	mediaTimescale     uint32 // From mdhd, used for stts frame rate calculation.
	currentHandlerType string // Handler type of current track ("vide", "soun", etc.). Reset per track.
	mdatOffset         int64  // Start offset of mdat box (0 if not seen).
	mdatSize           uint64 // Size of mdat box payload.
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
		case "meta":
			d.decodeMeta(startPos, boxSize)
		case "mdat":
			// Record mdat payload position/size for MediaDataOffset/MediaDataSize tags.
			// Exiftool reports the data start offset and data size (excluding header).
			// The current read position is just past the box header, so it's the data start.
			d.mdatOffset = d.pos()
			d.mdatSize = boxSize - uint64(d.pos()-startPos)
		case "free", "skip", "wide":
			// Skip padding boxes.
		default:
			// Skip unknown top-level boxes silently.
		}

		d.seekToBoxEnd(startPos, boxSize)
	}

	// Emit mdat metadata after all boxes are processed.
	if d.mdatOffset > 0 && d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("MediaDataSize", d.mdatSize)
		d.emitQuickTimeTag("MediaDataOffset", d.mdatOffset)
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

	switch size {
	case 1:
		// 64-bit extended size.
		totalSize = d.read8()
	case 0:
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

// decodeFtyp validates the ftyp box, sets isMOV flag, and emits brand tags.
func (d *videoDecoderMP4) decodeFtyp(startPos int64, boxSize uint64) error {
	majorBrand := d.readFourCC()
	minorVersion := d.read4()

	brandStr := majorBrand.String()

	// Read all compatible brands.
	endPos := startPos + int64(boxSize)
	var compatBrands []string
	for d.pos() < endPos {
		compat := d.readFourCC()
		compatBrands = append(compatBrands, compat.String())
	}

	// Validate brand before emitting tags — invalid files should not produce callbacks.
	if movBrands[brandStr] {
		d.isMOV = true
	} else if !mp4Brands[brandStr] {
		found := false
		for _, cb := range compatBrands {
			if movBrands[cb] {
				d.isMOV = true
				found = true
				break
			}
			if mp4Brands[cb] {
				found = true
				break
			}
		}
		if !found {
			return newInvalidFormatErrorf("unrecognized ftyp brand: %q", brandStr)
		}
	}

	// Emit ftyp tags after validation succeeds.
	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("MajorBrand", brandStr)
		// MinorVersion: exiftool formats as val/65536 . (val/256)%256 . val%256.
		d.emitQuickTimeTag("MinorVersion", fmt.Sprintf("%d.%d.%d",
			minorVersion>>16, (minorVersion>>8)&0xFF, minorVersion&0xFF))
		if len(compatBrands) > 0 {
			d.emitQuickTimeTag("CompatibleBrands", compatBrands)
		}
	}

	return nil
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
		case "uuid":
			d.decodeUUID(startPos, boxSize)
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

	// Store movie timescale for tkhd TrackDuration conversion.
	d.movieTimescale = timescale

	// Convert to time.Time and duration.
	if timescale > 0 {
		d.result.VideoConfig.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("MovieHeaderVersion", version)
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
	// Reset per-track state so audio/video detection starts fresh.
	d.currentHandlerType = ""

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
		case "tapt":
			d.decodeTapt(startPos, boxSize)
		case "mdia":
			d.decodeMdia(startPos, boxSize)
		case "tref":
			d.decodeTref(startPos, boxSize)
		case "uuid":
			d.decodeUUID(startPos, boxSize)
		case "meta":
			d.decodeMeta(startPos, boxSize)
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
		// tkhd duration is in movie timescale units; fall back to 1000 if mvhd not yet seen.
		trackTimescale := d.movieTimescale
		if trackTimescale == 0 {
			trackTimescale = 1000
		}
		d.emitQuickTimeTag("TrackDuration", durationSeconds(duration, trackTimescale))
		d.emitQuickTimeTag("TrackLayer", layer)
		d.emitQuickTimeTag("TrackVolume", fixedPoint88ToFloat(volume))
		// Only emit ImageWidth/ImageHeight for video tracks (audio tracks have zero dimensions).
		if width > 0 {
			d.emitQuickTimeTag("ImageWidth", width)
			d.emitQuickTimeTag("ImageHeight", height)
		}
		d.emitQuickTimeTag("MatrixStructure", formatMatrix(matrix))
	}
}

// decodeTapt parses the track aperture mode dimensions container box (QuickTime-specific).
// Contains clef, prof, and enof children for clean, production, and encoded dimensions.
func (d *videoDecoderMP4) decodeTapt(taptStart int64, taptSize uint64) {
	taptEnd := taptStart + int64(taptSize)
	for d.pos() < taptEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		switch boxType.String() {
		case "clef":
			d.decodeTaptDimension("CleanApertureDimensions")
		case "prof":
			d.decodeTaptDimension("ProductionApertureDimensions")
		case "enof":
			d.decodeTaptDimension("EncodedPixelsDimensions")
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeTaptDimension reads a tapt dimension box (clef/prof/enof).
// Layout: version+flags(4), width(4 fixed 16.16), height(4 fixed 16.16).
// Emits as "W H" string matching exiftool format.
func (d *videoDecoderMP4) decodeTaptDimension(tagName string) {
	_ = d.readBytes(4) // version + flags
	widthFixed := d.read4()
	heightFixed := d.read4()

	width := int(widthFixed >> 16)
	height := int(heightFixed >> 16)

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag(tagName, fmt.Sprintf("%d %d", width, height))
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
			d.decodeHdlr(startPos, boxSize)
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

	// Store for stts frame rate calculation.
	d.mediaTimescale = timescale

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("MediaHeaderVersion", version)
		d.emitQuickTimeTag("MediaCreateDate", quickTimeToTime(creationTime))
		d.emitQuickTimeTag("MediaModifyDate", quickTimeToTime(modificationTime))
		d.emitQuickTimeTag("MediaTimeScale", timescale)
		d.emitQuickTimeTag("MediaDuration", durationSeconds(duration, timescale))
		d.emitQuickTimeTag("MediaLanguageCode", lang)
	}
}

// decodeHdlr parses the handler reference box in mdia.
// Emits HandlerClass, HandlerType, and HandlerDescription.
func (d *videoDecoderMP4) decodeHdlr(hdlrStart int64, hdlrSize uint64) {
	_ = d.readBytes(4) // version + flags

	// pre_defined (Component type in QuickTime) — exiftool calls this HandlerClass.
	handlerClass := d.readFourCC()
	handlerType := d.readFourCC()

	// Track handler type for stsd branching. Only set from media handler types
	// (mdia hdlr), not from data handler references (minf hdlr has "alis" etc.).
	// Data handlers have well-known types like "alis", "url ", "rsrc".
	ht := handlerType.String()
	if ht != "alis" && ht != "url " && ht != "rsrc" {
		d.currentHandlerType = ht
	}

	// Read manufacturer/vendor (12 bytes = 3 x uint32 reserved).
	_ = d.readBytes(12)

	// Read handler description — rest of box is a null-terminated string.
	endPos := hdlrStart + int64(hdlrSize)
	nameLen := endPos - d.pos()
	var description string
	if nameLen > 0 && nameLen < 256 {
		nameBytes := d.readBytes(int(nameLen))
		description = printableString(string(trimNulls(nameBytes)))
	}

	if d.opts.Sources.Has(QUICKTIME) {
		hc := handlerClass.String()
		if hc != "\x00\x00\x00\x00" {
			d.emitQuickTimeTag("HandlerClass", hc)
		}
		d.emitQuickTimeTag("HandlerType", handlerType.String())
		if description != "" {
			d.emitQuickTimeTag("HandlerDescription", description)
		}
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

		switch boxType.String() {
		case "vmhd":
			d.decodeVmhd()
		case "smhd":
			d.decodeSmhd()
		case "hdlr":
			// QuickTime minf contains a data handler reference hdlr (comp_type "dhlr").
			d.decodeHdlr(startPos, boxSize)
		case "stbl":
			d.decodeStbl(startPos, boxSize)
		case "gmhd":
			d.decodeGmhd(startPos, boxSize)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeVmhd parses the video media header box.
func (d *videoDecoderMP4) decodeVmhd() {
	_ = d.readBytes(4) // version + flags
	graphicsMode := d.read2()
	r := d.read2()
	g := d.read2()
	b := d.read2()

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("GraphicsMode", graphicsMode)
		d.emitQuickTimeTag("OpColor", fmt.Sprintf("%d %d %d", r, g, b))
	}
}

// decodeSmhd parses the sound media header box.
func (d *videoDecoderMP4) decodeSmhd() {
	_ = d.readBytes(4) // version + flags
	balance := d.read2()
	_ = d.read2() // reserved

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("Balance", fixedPoint88ToFloat(balance))
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

		switch boxType.String() {
		case "stsd":
			d.decodeStsd(startPos, boxSize)
		case "stts":
			d.decodeStts()
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

// decodeStts parses the sample-to-time box to derive VideoFrameRate.
func (d *videoDecoderMP4) decodeStts() {
	_ = d.readBytes(4) // version + flags
	entryCount := d.read4()

	if entryCount == 0 || d.mediaTimescale == 0 || d.currentHandlerType != "vide" {
		return
	}

	if entryCount > 10000 {
		return // Sanity check.
	}

	// Read all entries to compute weighted average frame rate.
	var totalSamples uint64
	var totalDuration uint64
	for i := uint32(0); i < entryCount; i++ {
		sampleCount := d.read4()
		sampleDelta := d.read4()
		totalSamples += uint64(sampleCount)
		totalDuration += uint64(sampleCount) * uint64(sampleDelta)
	}

	if totalSamples > 0 && totalDuration > 0 {
		avgDelta := float64(totalDuration) / float64(totalSamples)
		frameRate := float64(d.mediaTimescale) / avgDelta
		if d.opts.Sources.Has(QUICKTIME) {
			d.emitQuickTimeTag("VideoFrameRate", frameRate)
		}
	}
}

// decodeStsd parses the sample description box, branching on handler type
// to parse either visual (video) or audio sample entries.
func (d *videoDecoderMP4) decodeStsd(stsdStart int64, stsdSize uint64) {
	_ = d.readBytes(4) // version + flags
	entryCount := d.read4()

	if entryCount == 0 {
		return
	}

	// Read first sample entry common header.
	entrySize := d.read4()
	codec := d.readFourCC()    // codec fourCC (e.g., "avc1", "hvc1", "mp4a")
	_ = d.readBytes(6)         // reserved
	_ = d.read2()              // data reference index
	entryStart := d.pos() - 16 // 4 (size) + 4 (type) + 6 (reserved) + 2 (data ref idx)

	switch d.currentHandlerType {
	case "soun":
		if d.opts.Sources.Has(QUICKTIME) {
			d.emitQuickTimeTag("AudioFormat", codec.String())
		}
		d.decodeAudioSampleEntry(entryStart, entrySize)
	case "vide":
		// Set codec in VideoConfig (first video track only).
		if d.result.VideoConfig.Codec == "" {
			d.result.VideoConfig.Codec = codec.String()
		}
		if d.opts.Sources.Has(QUICKTIME) {
			d.emitQuickTimeTag("CompressorID", codec.String())
		}
		d.decodeVisualSampleEntry(entryStart, entrySize)
	case "":
		// No handler seen yet — assume video for backwards compatibility with
		// files where hdlr appears after stsd (non-standard but possible).
		if d.result.VideoConfig.Codec == "" {
			d.result.VideoConfig.Codec = codec.String()
		}
		if d.opts.Sources.Has(QUICKTIME) {
			d.emitQuickTimeTag("CompressorID", codec.String())
		}
		d.decodeVisualSampleEntry(entryStart, entrySize)
	default:
		// Metadata tracks and other handler types — emit MetaFormat (codec fourCC).
		if d.opts.Sources.Has(QUICKTIME) {
			d.emitQuickTimeTag("MetaFormat", codec.String())
		}
	}
}

// decodeVisualSampleEntry parses the visual sample entry fields (ISO 14496-12 §12.1.3).
// The first 16 bytes after the common sample entry header are pre_defined/reserved in ISO,
// but in QuickTime they carry version(2), revision(2), vendor(4), temporal_quality(4),
// spatial_quality(4). We read vendor as VendorID for MOV files.
func (d *videoDecoderMP4) decodeVisualSampleEntry(entryStart int64, entrySize uint32) {
	_ = d.read2()              // version (pre_defined in ISO)
	_ = d.read2()              // revision (reserved in ISO)
	vendorID := d.readFourCC() // vendor (pre_defined in ISO)
	_ = d.readBytes(8)         // temporal_quality + spatial_quality (pre_defined in ISO)

	width := d.read2()
	height := d.read2()
	horizRes := d.read4() // 16.16 fixed point
	vertRes := d.read4()  // 16.16 fixed point
	_ = d.read4()         // reserved (data size)
	_ = d.read2()         // frame_count

	// CompressorName is a pascal string: first byte is length, then that many chars.
	var compNameBuf [32]byte
	d.readBytesInto(compNameBuf[:])
	nameLen := int(compNameBuf[0])
	compName := ""
	if nameLen > 0 && nameLen <= 31 {
		compName = printableString(string(compNameBuf[1 : 1+nameLen]))
	}

	bitDepth := d.read2()
	_ = d.read2() // pre_defined

	if d.opts.Sources.Has(QUICKTIME) {
		vid := vendorID.String()
		if vid != "\x00\x00\x00\x00" {
			d.emitQuickTimeTag("VendorID", vid)
		}
		d.emitQuickTimeTag("SourceImageWidth", int(width))
		d.emitQuickTimeTag("SourceImageHeight", int(height))
		d.emitQuickTimeTag("XResolution", fixedPoint1616ToInt(horizRes))
		d.emitQuickTimeTag("YResolution", fixedPoint1616ToInt(vertRes))
		if compName != "" {
			d.emitQuickTimeTag("CompressorName", compName)
		}
		d.emitQuickTimeTag("BitDepth", int(bitDepth))
	}

	// Parse sub-boxes within the sample entry (pasp, btrt, etc.).
	entryEnd := entryStart + int64(entrySize)
	for d.pos()+8 <= entryEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}

		switch subType.String() {
		case "pasp":
			d.decodePasp()
		case "btrt":
			d.decodeBtrt()
		}

		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeAudioSampleEntry parses the audio sample entry fields (ISO 14496-12 §12.2.3).
// Handles QuickTime V0 and V1 audio sample entries.
// Emits AudioChannels, AudioBitsPerSample, and AudioSampleRate.
func (d *videoDecoderMP4) decodeAudioSampleEntry(entryStart int64, entrySize uint32) {
	// QuickTime audio sample entry: version(2)+revision(2)+vendor(4) = 8 bytes.
	version := d.read2()
	_ = d.read2() // revision
	_ = d.read4() // vendor

	channelCount := d.read2()
	sampleSize := d.read2()
	_ = d.read2() // compression_id
	_ = d.read2() // packet_size

	// Sample rate is 16.16 fixed point.
	sampleRateFixed := d.read4()
	sampleRate := int(sampleRateFixed >> 16)

	// V1 has 4 extra uint32 fields after the standard ones.
	if version == 1 {
		_ = d.read4() // samplesPerPacket
		_ = d.read4() // bytesPerPacket
		_ = d.read4() // bytesPerFrame
		_ = d.read4() // bytesPerSample
	}

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("AudioChannels", int(channelCount))
		d.emitQuickTimeTag("AudioBitsPerSample", int(sampleSize))
		d.emitQuickTimeTag("AudioSampleRate", sampleRate)
	}

	// Parse sub-boxes (wave, esds, etc.) within the audio sample entry.
	entryEnd := entryStart + int64(entrySize)
	for d.pos()+8 <= entryEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}
		if subType.String() == "wave" {
			d.decodeWave(subStart, subSize)
		}
		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeWave parses the wave (sound data format) box inside audio sample entries.
// Extracts PurchaseFileFormat from the frma (original format) sub-box.
func (d *videoDecoderMP4) decodeWave(waveStart int64, waveSize uint64) {
	waveEnd := waveStart + int64(waveSize)
	for d.pos()+8 <= waveEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}
		if subType.String() == "frma" && subSize >= 12 {
			format := d.readFourCC()
			if d.opts.Sources.Has(QUICKTIME) {
				d.emitQuickTimeTag("PurchaseFileFormat", format.String())
			}
		}
		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodePasp parses the pixel aspect ratio box.
func (d *videoDecoderMP4) decodePasp() {
	hSpacing := d.read4()
	vSpacing := d.read4()
	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("PixelAspectRatio", fmt.Sprintf("%d:%d", hSpacing, vSpacing))
	}
}

// decodeBtrt parses the bitrate box.
func (d *videoDecoderMP4) decodeBtrt() {
	bufferSize := d.read4()
	maxBitrate := d.read4()
	avgBitrate := d.read4()
	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("BufferSize", bufferSize)
		d.emitQuickTimeTag("MaxBitrate", maxBitrate)
		d.emitQuickTimeTag("AverageBitrate", avgBitrate)
	}
}

// decodeUdta iterates the user data box.
// Handles meta containers, old-style QuickTime text atoms (©xxx), XMP_ boxes,
// Pentax TAGS atoms, and UUID boxes.
func (d *videoDecoderMP4) decodeUdta(udtaStart int64, udtaSize uint64) {
	udtaEnd := udtaStart + int64(udtaSize)
	for d.pos() < udtaEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		boxTypeStr := boxType.String()
		switch {
		case boxTypeStr == "meta":
			d.decodeMeta(startPos, boxSize)
		case boxTypeStr == "XMP_":
			// Raw XMP data directly in udta (common in QuickTime MOV files).
			if d.opts.Sources.Has(XMP) {
				dataLen := int(boxSize) - 8
				if dataLen > 0 {
					rc := d.bufferedReader(dataLen)
					_ = d.decodeXMP(rc)
					_ = rc.Close()
				}
			}
		case boxTypeStr == "TAGS":
			// Pentax manufacturer-specific binary metadata.
			if d.opts.Sources.Has(MAKERNOTES) {
				dataLen := int(boxSize) - 8
				if dataLen > 0 && dataLen < 1024*1024 {
					data := d.readBytes(dataLen)
					d.decodePentaxTAGS(data)
				}
			}
		case boxTypeStr == "uuid":
			d.decodeUUID(startPos, boxSize)
		case len(boxTypeStr) > 0 && boxTypeStr[0] == '\xa9':
			// Old-style QuickTime text atom: 2-byte text_size, 2-byte language, then text.
			// These appear in udta with types like ©fmt, ©inf, etc.
			if d.opts.Sources.Has(QUICKTIME) {
				dataLen := int(boxSize) - 8
				if dataLen >= 4 {
					textSize := int(d.read2())
					_ = d.read2() // language code
					if textSize > 0 && textSize <= dataLen-4 {
						text := printableString(string(d.readBytes(textSize)))
						tagName := ilstAtomToTagName(boxTypeStr)
						if tagName != "" {
							d.emitQuickTimeTag(tagName, text)
						}
					}
				}
			}
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
		case "idat":
			// Sony NRTM: XML may be embedded in idat within the meta box.
			if handlerType == "nrtm" && d.opts.Sources.Has(XML) {
				dataLen := int(boxSize) - 8
				if dataLen > 0 && dataLen < 1024*1024 {
					data := d.readBytes(dataLen)
					xmlData := scanForXMLInMeta(data)
					if xmlData != nil {
						d.decodeSonyNRTM(bytes.NewReader(xmlData))
					}
				}
			}
		case "xml ":
			// XML sub-box (FullBox): version+flags(4) + XML data.
			if handlerType == "nrtm" && d.opts.Sources.Has(XML) {
				_ = d.readBytes(4) // version + flags
				dataLen := int(boxSize) - 12
				if dataLen > 0 && dataLen < 1024*1024 {
					data := d.readBytes(dataLen)
					xmlData := scanForXMLInMeta(data)
					if xmlData != nil {
						d.decodeSonyNRTM(bytes.NewReader(xmlData))
					}
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
		keySize := d.read4()        // Size including this 4 bytes + namespace(4) + key string.
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
			defer func() { _ = rc.Close() }()
			_ = d.decodeXMP(rc)
		}
	case exifUUID:
		if d.opts.Sources.Has(EXIF) {
			// EXIF in MP4 UUID box has a 4-byte header offset prefix.
			headerOffset := d.read4()
			rc := d.bufferedReader(int(dataLen) - 4)
			defer func() { _ = rc.Close() }()
			// Skip the header offset bytes within the buffered reader.
			if headerOffset > 0 {
				buf := make([]byte, headerOffset)
				_, _ = io.ReadFull(rc, buf)
			}
			d.decodeEXIF(rc)
		}
	case profUUID:
		if d.opts.Sources.Has(QUICKTIME) {
			d.decodePROFUUID(dataLen)
		}
	case usmtUUID:
		if d.opts.Sources.Has(QUICKTIME) {
			d.decodeUSMTUUID(dataLen)
		}
	default:
		// Unknown UUID — silently skip.
	}
}

// emitQuickTimeTag sends a QuickTime source tag via the centralized emitTag.
func (d *videoDecoderMP4) emitQuickTimeTag(name string, value any) {
	d.emitTag(TagInfo{
		Source:    QUICKTIME,
		Tag:       name,
		Namespace: "QuickTime",
		Value:     value,
	})
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
// Elements [0..5] are 16.16 fixed point (>>16 for integer).
// Elements [6..8] are 2.30 fixed point (>>30 for integer).
func formatMatrix(m [9]int32) string {
	return fmt.Sprintf("%d %d %d %d %d %d %d %d %d",
		m[0]>>16, m[1]>>16, m[2]>>16,
		m[3]>>16, m[4]>>16, m[5]>>16,
		m[6]>>30, m[7]>>30, m[8]>>30,
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

// isASCIIFourCC returns true if all 4 bytes are printable ASCII (0x20-0x7E).
func isASCIIFourCC(fcc fourCC) bool {
	for _, b := range fcc {
		if b < 0x20 || b > 0x7E {
			return false
		}
	}
	return true
}

// decodePROFUUID parses the Sony XAVC UUID-PROF box containing file, video,
// and audio profile sub-boxes (FPRF, VPRF, APRF).
func (d *videoDecoderMP4) decodePROFUUID(dataLen int64) {
	endPos := d.pos() + dataLen
	_ = d.read4() // version
	_ = d.read4() // profile count

	for d.pos()+8 <= endPos {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}

		switch subType.String() {
		case "FPRF":
			d.decodeFPRF()
		case "VPRF":
			d.decodeVPRF()
		case "APRF":
			d.decodeAPRF()
		}

		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeFPRF parses the file profile sub-box of UUID-PROF.
func (d *videoDecoderMP4) decodeFPRF() {
	_ = d.read4() // version
	flags := d.read4()
	_ = d.read4() // reserved
	d.emitQuickTimeTag("FileFunctionFlags", flags)
}

// decodeVPRF parses the video profile sub-box of UUID-PROF.
// Layout: version(4) + trackID(4) + codec(4) + unknown(4) + attributes(4) +
// avgBitrate(4) + maxBitrate(4) + avgFrameRate(4) + maxFrameRate(4) +
// size(2+2) + pixelAspect(2+2).
func (d *videoDecoderMP4) decodeVPRF() {
	_ = d.read4() // version
	trackID := d.read4()
	codec := d.readFourCC()
	_ = d.read4() // codec info (unknown)
	attributes := d.read4()
	avgBitrate := d.read4()
	maxBitrate := d.read4()
	avgFrameRateFixed := d.read4()
	maxFrameRateFixed := d.read4()

	// VideoSize: 2 × uint16 packed in 4 bytes.
	w := d.read2()
	h := d.read2()
	// PixelAspectRatio: 2 × uint16.
	parH := d.read2()
	parV := d.read2()

	d.emitQuickTimeTag("VideoTrackID", trackID)
	d.emitQuickTimeTag("VideoCodec", codec.String())
	d.emitQuickTimeTag("VideoAttributes", attributes)
	// Bitrates are stored in kbps; exiftool multiplies by 1000 for bps.
	d.emitQuickTimeTag("VideoAvgBitrate", int(avgBitrate)*1000)
	d.emitQuickTimeTag("VideoMaxBitrate", int(maxBitrate)*1000)
	// Frame rates are fixed-point 16.16.
	d.emitQuickTimeTag("VideoAvgFrameRate", float64(avgFrameRateFixed)/65536.0)
	d.emitQuickTimeTag("VideoMaxFrameRate", float64(maxFrameRateFixed)/65536.0)
	d.emitQuickTimeTag("VideoSize", fmt.Sprintf("%d %d", w, h))
	d.emitQuickTimeTag("PixelAspectRatio", fmt.Sprintf("%d %d", parH, parV))
}

// decodeAPRF parses the audio profile sub-box of UUID-PROF.
func (d *videoDecoderMP4) decodeAPRF() {
	_ = d.read4() // version
	trackID := d.read4()
	codec := d.readFourCC()
	_ = d.read4() // codec info (unknown)
	attributes := d.read4()
	avgBitrate := d.read4()
	maxBitrate := d.read4()
	sampleRate := d.read4()
	channels := d.read4()

	d.emitQuickTimeTag("AudioTrackID", trackID)
	d.emitQuickTimeTag("AudioCodec", codec.String())
	d.emitQuickTimeTag("AudioAttributes", attributes)
	d.emitQuickTimeTag("AudioAvgBitrate", int(avgBitrate)*1000)
	d.emitQuickTimeTag("AudioMaxBitrate", int(maxBitrate)*1000)
	d.emitQuickTimeTag("AudioSampleRate", sampleRate)
	d.emitQuickTimeTag("AudioChannels", channels)
}

// decodeUSMTUUID parses the Sony UUID-USMT box containing MTDT metadata entries.
func (d *videoDecoderMP4) decodeUSMTUUID(dataLen int64) {
	endPos := d.pos() + dataLen
	for d.pos()+8 <= endPos {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}
		if subType.String() == "MTDT" {
			d.decodeMTDT(int(subSize) - 8)
		}
		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeMTDT parses a Sony MTDT (MetaData) box.
// Format: entry_count(2), per entry: data_size(2) + data_type(4) + language(2) + encoding(2) + data(N).
func (d *videoDecoderMP4) decodeMTDT(payloadLen int) {
	if payloadLen < 2 {
		return
	}
	entryCount := d.read2()
	for i := 0; i < int(entryCount); i++ {
		dataSize := d.read2()
		if dataSize < 10 {
			break
		}
		dataType := d.read4()
		_ = d.read2() // language
		_ = d.read2() // encoding (1=UTF-16BE, 0=binary)

		valueLen := int(dataSize) - 10
		if valueLen <= 0 {
			continue
		}

		switch dataType {
		case 0x000a:
			// TrackProperty: 8 bytes — uint32 + uint16 + uint16 (space-separated).
			if valueLen >= 8 {
				propData := d.readBytes(8)
				d.emitQuickTimeTag("TrackProperty", fmt.Sprintf("%d %d %d",
					binary.BigEndian.Uint32(propData[0:4]),
					binary.BigEndian.Uint16(propData[4:6]),
					binary.BigEndian.Uint16(propData[6:8])))
				if valueLen > 8 {
					d.skip(int64(valueLen - 8))
				}
			} else {
				d.skip(int64(valueLen))
			}
		case 0x000b:
			// TimeZone: 2-byte signed integer (minutes offset from UTC).
			if valueLen >= 2 {
				tz := int16(d.read2())
				d.emitQuickTimeTag("TimeZone", int(tz))
				if valueLen > 2 {
					d.skip(int64(valueLen - 2))
				}
			} else {
				d.skip(int64(valueLen))
			}
		default:
			d.skip(int64(valueLen))
		}
	}
}

// decodeTref iterates the track reference box, handling cdsc (content describes).
func (d *videoDecoderMP4) decodeTref(trefStart int64, trefSize uint64) {
	trefEnd := trefStart + int64(trefSize)
	for d.pos() < trefEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}
		if subType.String() == "cdsc" {
			// cdsc contains one or more track IDs that this track describes.
			if subSize >= 12 {
				trackID := d.read4()
				if d.opts.Sources.Has(QUICKTIME) {
					d.emitQuickTimeTag("ContentDescribes", trackID)
				}
			}
		}
		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeGmhd parses the generic media header container (QuickTime-specific).
func (d *videoDecoderMP4) decodeGmhd(gmhdStart int64, gmhdSize uint64) {
	gmhdEnd := gmhdStart + int64(gmhdSize)
	for d.pos() < gmhdEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF || subSize < 8 {
			break
		}
		if subType.String() == "gmin" {
			d.decodeGmin()
		}
		d.seekToBoxEnd(subStart, subSize)
	}
}

// decodeGmin parses the generic media information header (gmin) box.
func (d *videoDecoderMP4) decodeGmin() {
	version := d.read1()
	flags := d.readBytes(3)
	graphicsMode := d.read2()
	r := d.read2()
	g := d.read2()
	b := d.read2()
	balance := d.read2()
	_ = d.read2() // reserved

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("GenMediaVersion", int(version))
		d.emitQuickTimeTag("GenFlags", fmt.Sprintf("%d %d %d", flags[0], flags[1], flags[2]))
		d.emitQuickTimeTag("GenGraphicsMode", int(graphicsMode))
		d.emitQuickTimeTag("GenOpColor", fmt.Sprintf("%d %d %d", r, g, b))
		d.emitQuickTimeTag("GenBalance", fixedPoint88ToFloat(balance))
	}
}
