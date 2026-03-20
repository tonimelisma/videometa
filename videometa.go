// Package videometa reads metadata from video files.
//
// It extracts EXIF, XMP, IPTC, and QuickTime native metadata from
// MP4/MOV containers (ISOBMFF format). All output matches exiftool -n -json.
package videometa

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"
)

// Source identifies where a metadata tag originated.
type Source uint32

const (
	EXIF       Source = 1 << iota // EXIF IFD data
	XMP                           // XMP/RDF XML
	IPTC                          // IPTC-IIM records
	QUICKTIME                     // QuickTime native metadata (ilst, freeform atoms)
	CONFIG                        // Codec/dimension info from container structure
	MAKERNOTES                    // Manufacturer-specific metadata (Pentax TAGS, etc.)
	XML                           // Structured XML metadata (Sony NRTM, etc.)
	COMPOSITE                     // Derived/computed tags (matching exiftool Composite group)
)

// Has reports whether s contains the given source.
func (s Source) Has(source Source) bool { return s&source != 0 }

// Remove clears the given source from s.
func (s Source) Remove(source Source) Source { return s &^ source }

// IsZero reports whether no sources are set.
func (s Source) IsZero() bool { return s == 0 }

// VideoFormat identifies the container format.
type VideoFormat int

const (
	// MP4 covers both MP4 and MOV containers (auto-detected from ftyp brand).
	MP4 VideoFormat = iota + 1
)

// HandleTagFunc is called for each metadata tag found during decoding.
// Return ErrStopWalking to stop early.
type HandleTagFunc func(TagInfo) error

// TagInfo represents a single metadata tag.
type TagInfo struct {
	Source    Source
	Tag       string // Tag name matching exiftool output
	Namespace string // Source-specific grouping (IFD name, XMP namespace URI, etc.)
	Value     any
}

// Options configures a Decode or DecodeAll call.
type Options struct {
	// R is the video data source. io.ReadSeeker preferred; io.Reader accepted
	// with degraded performance (cannot seek past mdat).
	R io.Reader

	// VideoFormat identifies the container. If zero, auto-detected from ftyp.
	VideoFormat VideoFormat

	// Sources selects which metadata sources to extract.
	// Zero value means all sources.
	Sources Source

	// HandleTag is called for each decoded tag. Required for Decode().
	HandleTag HandleTagFunc

	// ShouldHandleTag filters tags before HandleTag. Return false to skip.
	ShouldHandleTag func(TagInfo) bool

	// HandleXMP, if set, receives the raw XMP reader instead of parsing it.
	HandleXMP func(r io.Reader) error

	// Warnf receives non-fatal warnings (e.g., skipped boxes, partial decodes).
	Warnf func(format string, args ...any)

	// Timeout bounds total decode time. Zero means no timeout.
	// Warning: when a timeout fires, DecodeResult may contain partial data
	// because the decode goroutine continues running briefly after cancellation.
	Timeout time.Duration

	// LimitNumTags caps total tags delivered. Default 5000.
	LimitNumTags uint32

	// LimitTagSize caps individual tag value size in bytes. Default 10000.
	LimitTagSize uint32
}

// VideoConfig holds codec and dimension info extracted from the container.
type VideoConfig struct {
	Width    int
	Height   int
	Duration time.Duration
	Rotation int    // Degrees clockwise (0, 90, 180, 270)
	Codec    string // FourCC code (e.g., "avc1", "hvc1")
}

// DecodeResult is returned by Decode.
type DecodeResult struct {
	VideoConfig VideoConfig
}

// ErrStopWalking can be returned from HandleTag to stop decoding early.
var ErrStopWalking = errors.New("stop walking")

// Decode reads metadata from a video file, calling opts.HandleTag for each tag.
func Decode(opts Options) (result DecodeResult, err error) {
	if opts.R == nil {
		return result, fmt.Errorf("videometa: Options.R is required")
	}
	if opts.HandleTag == nil {
		return result, fmt.Errorf("videometa: Options.HandleTag is required")
	}
	if opts.Sources.IsZero() {
		opts.Sources = EXIF | XMP | IPTC | QUICKTIME | CONFIG | MAKERNOTES | XML
	}
	if opts.LimitNumTags == 0 {
		opts.LimitNumTags = 5000
	}
	if opts.LimitTagSize == 0 {
		opts.LimitTagSize = 10000
	}

	// Wrap reader in streamReader.
	sr := newStreamReader(opts.R)

	// Recover panics from streamReader's stop() calls and HandleTag errors.
	defer func() {
		if r := recover(); r != nil {
			if r == errStop {
				if sr.readErr != nil {
					// Errors already wrapped as InvalidFormatError at the
					// source (readFull, readBytes, bufferedReader) pass
					// through directly. Other errors (seek, skip) propagate
					// as-is.
					err = sr.readErr
				}
			} else if e, ok := r.(error); ok && errors.Is(e, ErrStopWalking) {
				// ErrStopWalking panicked from HandleTag callback — not an error.
				err = nil
			} else if e, ok := r.(error); ok {
				// Other errors panicked from HandleTag — propagate.
				err = e
			} else {
				// Re-panic for unexpected panics.
				panic(r)
			}
		}
	}()

	bd := &baseDecoder{
		streamReader: sr,
		opts:         opts,
		result:       &result,
	}

	// Auto-detect format if not specified.
	format := opts.VideoFormat
	if format == 0 {
		format = MP4 // Only format we support in v1.
	}

	var dec decoder
	switch format {
	case MP4:
		dec = newVideoDecoderMP4(bd)
	default:
		return result, fmt.Errorf("videometa: unsupported format %d", format)
	}

	// Run decode with optional timeout.
	if opts.Timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- dec.decode()
		}()
		select {
		case decErr := <-done:
			if decErr != nil && !errors.Is(decErr, ErrStopWalking) {
				return result, decErr
			}
		case <-time.After(opts.Timeout):
			return result, fmt.Errorf("videometa: decode timed out after %v", opts.Timeout)
		}
	} else {
		if decErr := dec.decode(); decErr != nil {
			if errors.Is(decErr, ErrStopWalking) {
				return result, nil
			}
			return result, decErr
		}
	}

	return result, nil
}

// Tags collects decoded metadata for convenient access via DecodeAll.
type Tags struct {
	exif       map[string]TagInfo
	xmp        map[string]TagInfo
	iptc       map[string]TagInfo
	quicktime  map[string]TagInfo
	config     map[string]TagInfo
	makernotes map[string]TagInfo
	xml        map[string]TagInfo
	composite  map[string]TagInfo
}

// Add stores a tag in the appropriate source map.
func (t *Tags) Add(tag TagInfo) {
	var m *map[string]TagInfo
	switch tag.Source {
	case EXIF:
		m = &t.exif
	case XMP:
		m = &t.xmp
	case IPTC:
		m = &t.iptc
	case QUICKTIME:
		m = &t.quicktime
	case CONFIG:
		m = &t.config
	case MAKERNOTES:
		m = &t.makernotes
	case XML:
		m = &t.xml
	case COMPOSITE:
		m = &t.composite
	default:
		return
	}
	if *m == nil {
		*m = make(map[string]TagInfo)
	}
	(*m)[tag.Tag] = tag
}

// All returns all tags merged into a single map. On key collision,
// priority is EXIF > XMP > QUICKTIME > MAKERNOTES > XML > IPTC > CONFIG.
func (t Tags) All() map[string]TagInfo {
	result := make(map[string]TagInfo)
	// Lowest priority first, highest last (overwrites).
	for _, m := range []map[string]TagInfo{t.composite, t.config, t.iptc, t.xml, t.makernotes, t.quicktime, t.xmp, t.exif} {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// EXIF returns all EXIF tags.
func (t Tags) EXIF() map[string]TagInfo { return t.exif }

// XMP returns all XMP tags.
func (t Tags) XMP() map[string]TagInfo { return t.xmp }

// IPTC returns all IPTC tags.
func (t Tags) IPTC() map[string]TagInfo { return t.iptc }

// QuickTime returns all QuickTime native tags.
func (t Tags) QuickTime() map[string]TagInfo { return t.quicktime }

// Config returns all CONFIG tags.
func (t Tags) Config() map[string]TagInfo { return t.config }

// MakerNotes returns all manufacturer-specific metadata tags.
func (t Tags) MakerNotes() map[string]TagInfo { return t.makernotes }

// XML returns all structured XML metadata tags (e.g., Sony NRTM).
func (t Tags) XML() map[string]TagInfo { return t.xml }

// Composite returns all derived/computed tags.
func (t Tags) Composite() map[string]TagInfo { return t.composite }

// GetDateTime returns the best available creation time with original timezone.
// Priority: EXIF DateTimeOriginal > XMP CreateDate > QuickTime CreationDate > QuickTime CreateDate.
func (t Tags) GetDateTime() (time.Time, error) {
	// Try sources in priority order.
	candidates := []struct {
		tags map[string]TagInfo
		keys []string
	}{
		{t.exif, []string{"DateTimeOriginal", "CreateDate", "ModifyDate"}},
		{t.xmp, []string{"DateTimeOriginal", "CreateDate", "ModifyDate"}},
		{t.quicktime, []string{"CreationDate", "CreateDate", "ModifyDate"}},
		{t.iptc, []string{"DateCreated"}},
	}

	for _, c := range candidates {
		for _, key := range c.keys {
			if tag, ok := c.tags[key]; ok {
				if dt, err := parseAnyDateTime(tag.Value); err == nil {
					return dt, nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("videometa: no date/time found")
}

// GetDateTimeUTC returns GetDateTime() normalized to UTC.
func (t Tags) GetDateTimeUTC() (time.Time, error) {
	dt, err := t.GetDateTime()
	if err != nil {
		return time.Time{}, err
	}
	return dt.UTC(), nil
}

// GetLatLong returns GPS coordinates in decimal degrees.
// Priority: EXIF GPS > XMP GPS > QuickTime GPS.
func (t Tags) GetLatLong() (lat, lon float64, err error) {
	// Try EXIF GPS.
	if latTag, ok := t.exif["GPSLatitude"]; ok {
		if lonTag, ok := t.exif["GPSLongitude"]; ok {
			if latVal, ok := toFloat64(latTag.Value); ok {
				if lonVal, ok := toFloat64(lonTag.Value); ok {
					return latVal, lonVal, nil
				}
			}
		}
	}

	// Try XMP GPS.
	if latTag, ok := t.xmp["GPSLatitude"]; ok {
		if lonTag, ok := t.xmp["GPSLongitude"]; ok {
			if latVal, ok := toFloat64(latTag.Value); ok {
				if lonVal, ok := toFloat64(lonTag.Value); ok {
					return latVal, lonVal, nil
				}
			}
		}
	}

	// Try QuickTime GPS (space-separated decimal or ISO6709).
	if gpsTag, ok := t.quicktime["GPSCoordinates"]; ok {
		if s, ok := gpsTag.Value.(string); ok {
			if lat, lon, err := parseGPSCoordinatesString(s); err == nil {
				return lat, lon, nil
			}
		}
	}

	return 0, 0, fmt.Errorf("videometa: no GPS coordinates found")
}

// DecodeAll decodes all metadata into a Tags struct and returns the DecodeResult
// containing VideoConfig (width, height, duration, codec, rotation).
func DecodeAll(opts Options) (Tags, DecodeResult, error) {
	var tags Tags
	opts.HandleTag = func(ti TagInfo) error {
		tags.Add(ti)
		return nil
	}
	result, err := Decode(opts)
	computeComposite(&tags, result)
	return tags, result, err
}

// decoder is the internal interface for format-specific decoders.
type decoder interface {
	decode() error
}

// baseDecoder provides shared state for all format decoders.
type baseDecoder struct {
	*streamReader
	opts     Options
	result   *DecodeResult
	tagCount uint32 // Number of tags emitted so far.
}

// emitTag is the centralized tag emission method. All source-specific emit
// methods must delegate to this. It enforces LimitNumTags and LimitTagSize.
func (bd *baseDecoder) emitTag(ti TagInfo) {
	if bd.opts.HandleTag == nil {
		return
	}
	if bd.opts.ShouldHandleTag != nil && !bd.opts.ShouldHandleTag(ti) {
		return
	}

	// Enforce LimitTagSize: skip oversized tags silently (like imagemeta).
	if bd.opts.LimitTagSize > 0 {
		switch v := ti.Value.(type) {
		case string:
			if uint32(len(v)) > bd.opts.LimitTagSize {
				return
			}
		case []byte:
			if uint32(len(v)) > bd.opts.LimitTagSize {
				return
			}
		}
	}

	// Enforce LimitNumTags: stop decoding after limit.
	bd.tagCount++
	if bd.opts.LimitNumTags > 0 && bd.tagCount > bd.opts.LimitNumTags {
		panic(ErrStopWalking)
	}

	if err := bd.opts.HandleTag(ti); err != nil {
		panic(err)
	}
}

// newVideoDecoderMP4 creates the MP4/MOV decoder.
// Stub — implemented in videodecoder_mp4.go.
func newVideoDecoderMP4(bd *baseDecoder) decoder {
	return &videoDecoderMP4{baseDecoder: bd}
}

// computeComposite derives Composite tags from already-decoded data,
// matching exiftool's Composite group output.
func computeComposite(tags *Tags, result DecodeResult) {
	add := func(name string, value any) {
		tags.Add(TagInfo{Source: COMPOSITE, Tag: name, Namespace: "Composite", Value: value})
	}

	w := result.VideoConfig.Width
	h := result.VideoConfig.Height

	if w > 0 && h > 0 {
		add("ImageSize", fmt.Sprintf("%d %d", w, h))
		add("Megapixels", float64(w*h)/1000000.0)
	}

	add("Rotation", result.VideoConfig.Rotation)

	// AvgBitrate: MediaDataSize * 8 / Duration.
	qt := tags.QuickTime()
	if mdSize, ok := qt["MediaDataSize"]; ok {
		if dur, ok := qt["Duration"]; ok {
			if sizeF, ok := toFloat64(mdSize.Value); ok {
				if durF, ok := toFloat64(dur.Value); ok && durF > 0 {
					add("AvgBitrate", int(math.Round(sizeF*8/durF)))
				}
			}
		}
	}

	// GPS decomposition from QuickTime GPSCoordinates (space-separated decimal).
	if gpsTag, ok := qt["GPSCoordinates"]; ok {
		if s, ok := gpsTag.Value.(string); ok {
			lat, lon, err := parseGPSCoordinatesString(s)
			if err == nil {
				add("GPSLatitude", lat)
				add("GPSLongitude", lon)
				add("GPSPosition", fmt.Sprintf("%g %g", lat, lon))
			}
			alt, altOk := parseGPSAltitudeFromString(s)
			if altOk {
				ref := 0
				if alt < 0 {
					ref = 1
					alt = -alt
				}
				add("GPSAltitude", alt)
				add("GPSAltitudeRef", ref)
			}
		}
	}

	// Photography composites from MakerNotes.
	mn := tags.MakerNotes()

	if fn, ok := mn["FNumber"]; ok {
		if v, ok := toFloat64(fn.Value); ok {
			add("Aperture", v)
		}
	}

	if et, ok := mn["ExposureTime"]; ok {
		if v, ok := toFloat64(et.Value); ok {
			add("ShutterSpeed", v)
		}
	}

	if fl, ok := mn["FocalLength"]; ok {
		if v, ok := toFloat64(fl.Value); ok {
			add("FocalLength35efl", v)
		}
	}

	// LightValue: log2(Aperture^2 / ShutterSpeed) - log2(ISO/100).
	if apTag, ok := mn["FNumber"]; ok {
		if etTag, ok := mn["ExposureTime"]; ok {
			if isoTag, ok := mn["ISO"]; ok {
				ap, _ := toFloat64(apTag.Value)
				et, _ := toFloat64(etTag.Value)
				iso, _ := toFloat64(isoTag.Value)
				if ap > 0 && et > 0 && iso > 0 {
					lv := math.Log2(ap*ap/et) - math.Log2(iso/100)
					add("LightValue", lv)
				}
			}
		}
	}

	// LensID from QuickTime freeform LensModel.
	if lm, ok := qt["LensModel"]; ok {
		add("LensID", lm.Value)
	}
}
