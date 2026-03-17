// Package videometa reads metadata from video files.
//
// It extracts EXIF, XMP, IPTC, and QuickTime native metadata from
// MP4/MOV containers (ISOBMFF format). All output matches exiftool -n -json.
package videometa

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// Source identifies where a metadata tag originated.
type Source uint32

const (
	EXIF      Source = 1 << iota // EXIF IFD data
	XMP                         // XMP/RDF XML
	IPTC                        // IPTC-IIM records
	QUICKTIME                   // QuickTime native metadata (ilst, freeform atoms)
	CONFIG                      // Codec/dimension info from container structure
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
	// Zero value means EXIF|XMP|IPTC|QUICKTIME|CONFIG.
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
		opts.Sources = EXIF | XMP | IPTC | QUICKTIME | CONFIG
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
					if isInvalidFormatErrorCandidate(sr.readErr) {
						err = &InvalidFormatError{Err: sr.readErr}
					} else {
						err = sr.readErr
					}
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

	if decErr := dec.decode(); decErr != nil {
		if errors.Is(decErr, ErrStopWalking) {
			return result, nil
		}
		return result, decErr
	}

	return result, nil
}

// Tags collects decoded metadata for convenient access via DecodeAll.
type Tags struct {
	exif      map[string]TagInfo
	xmp       map[string]TagInfo
	iptc      map[string]TagInfo
	quicktime map[string]TagInfo
	config    map[string]TagInfo
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
	default:
		return
	}
	if *m == nil {
		*m = make(map[string]TagInfo)
	}
	(*m)[tag.Tag] = tag
}

// All returns all tags merged into a single map. On key collision,
// priority is EXIF > XMP > QUICKTIME > IPTC > CONFIG.
func (t Tags) All() map[string]TagInfo {
	result := make(map[string]TagInfo)
	// Lowest priority first, highest last (overwrites).
	for _, m := range []map[string]TagInfo{t.config, t.iptc, t.quicktime, t.xmp, t.exif} {
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

// GetDateTime returns the best available creation time with original timezone.
// Priority: EXIF DateTimeOriginal > XMP DateTimeOriginal > QuickTime CreationDate > IPTC DateCreated.
func (t Tags) GetDateTime() (time.Time, error) {
	// Will be implemented in Milestone 7.
	return time.Time{}, fmt.Errorf("videometa: GetDateTime not yet implemented")
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
	// Will be implemented in Milestone 7.
	return 0, 0, fmt.Errorf("videometa: GetLatLong not yet implemented")
}

// DecodeAll decodes all metadata into a Tags struct.
func DecodeAll(opts Options) (Tags, error) {
	var tags Tags
	opts.HandleTag = func(ti TagInfo) error {
		tags.Add(ti)
		return nil
	}
	_, err := Decode(opts)
	return tags, err
}

// decoder is the internal interface for format-specific decoders.
type decoder interface {
	decode() error
}

// baseDecoder provides shared state for all format decoders.
type baseDecoder struct {
	*streamReader
	opts   Options
	result *DecodeResult
}

// newVideoDecoderMP4 creates the MP4/MOV decoder.
// Stub — implemented in videodecoder_mp4.go.
func newVideoDecoderMP4(bd *baseDecoder) decoder {
	return &videoDecoderMP4{baseDecoder: bd}
}
