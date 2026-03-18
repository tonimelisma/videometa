package videometa

import (
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
		case "mdat":
			// Record mdat position/size for MediaDataOffset/MediaDataSize tags.
			d.mdatOffset = startPos
			// Box size includes 8-byte header; payload is the rest.
			d.mdatSize = boxSize - 8
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

	if entryCount == 0 || d.mediaTimescale == 0 {
		return
	}

	// Read first entry.
	_ = d.read4()            // sample_count
	sampleDelta := d.read4() // sample_delta

	// Only emit frame rate for constant-rate video (single entry with non-zero delta).
	if entryCount == 1 && sampleDelta > 0 {
		frameRate := float64(d.mediaTimescale) / float64(sampleDelta)
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
		// Other handler types (e.g., metadata tracks) — skip.
	}
}

// decodeVisualSampleEntry parses the visual sample entry fields (ISO 14496-12 §12.1.3).
func (d *videoDecoderMP4) decodeVisualSampleEntry(entryStart int64, entrySize uint32) {
	_ = d.readBytes(2)  // pre_defined
	_ = d.readBytes(2)  // reserved
	_ = d.readBytes(12) // pre_defined

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
		d.emitQuickTimeTag("SourceImageWidth", int(width))
		d.emitQuickTimeTag("SourceImageHeight", int(height))
		d.emitQuickTimeTag("XResolution", fixedPoint1616ToInt(horizRes))
		d.emitQuickTimeTag("YResolution", fixedPoint1616ToInt(vertRes))
		if compName != "" {
			d.emitQuickTimeTag("CompressorName", compName)
		}
		d.emitQuickTimeTag("BitDepth", int(bitDepth))
	}

	// TODO: Parse VendorID from QuickTime visual sample entry extension
	// (4 bytes after pre_defined, present in MOV but not ISO MP4).
	// TODO: Parse clap/prod/encd sub-boxes for CleanApertureDimensions,
	// ProductionApertureDimensions, EncodedPixelsDimensions.

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
// Emits AudioChannels, AudioBitsPerSample, and AudioSampleRate.
func (d *videoDecoderMP4) decodeAudioSampleEntry(entryStart int64, entrySize uint32) {
	_ = d.readBytes(8) // reserved

	channelCount := d.read2()
	sampleSize := d.read2()
	_ = d.read2() // pre_defined (compression_id)
	_ = d.read2() // reserved (packet_size)

	// Sample rate is 16.16 fixed point.
	sampleRateFixed := d.read4()
	sampleRate := int(sampleRateFixed >> 16)

	if d.opts.Sources.Has(QUICKTIME) {
		d.emitQuickTimeTag("AudioChannels", int(channelCount))
		d.emitQuickTimeTag("AudioBitsPerSample", int(sampleSize))
		d.emitQuickTimeTag("AudioSampleRate", sampleRate)
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
	default:
		if d.opts.Warnf != nil {
			d.opts.Warnf("decode uuid: unknown UUID box")
		}
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
