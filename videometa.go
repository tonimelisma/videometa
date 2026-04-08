// Package videometa reads metadata from video files.
//
// It extracts QuickTime and vendor-specific container metadata from MP4/MOV
// containers (ISOBMFF format). All output matches exiftool for the supported
// video-native metadata paths, with grouped and ordered golden tests enforcing
// parity.
package videometa

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
)

// Source identifies where a metadata tag originated.
type Source uint32

const (
	QUICKTIME Source = 1 << iota // Standard/container-native QuickTime metadata
	VENDOR                       // Vendor-specific container metadata families
	CONFIG                       // VideoConfig request flag; not emitted as tags
	COMPOSITE                    // Derived/computed tags, only materialized by DecodeAll
)

// Has reports whether s contains the given source.
func (s Source) Has(source Source) bool { return s&source != 0 }

// Remove clears the given source from s.
func (s Source) Remove(source Source) Source { return s &^ source }

// IsZero reports whether no sources are set.
func (s Source) IsZero() bool { return s == 0 }

// String returns a stable, human-readable representation of the source set.
func (s Source) String() string {
	if s == 0 {
		return "0"
	}

	names := []struct {
		source Source
		name   string
	}{
		{source: QUICKTIME, name: "QUICKTIME"},
		{source: VENDOR, name: "VENDOR"},
		{source: CONFIG, name: "CONFIG"},
		{source: COMPOSITE, name: "COMPOSITE"},
	}

	remaining := s
	parts := make([]string, 0, len(names))
	for _, item := range names {
		if remaining.Has(item.source) {
			parts = append(parts, item.name)
			remaining = remaining.Remove(item.source)
		}
	}
	if remaining != 0 {
		parts = append(parts, fmt.Sprintf("Source(0x%x)", uint32(remaining)))
	}
	return strings.Join(parts, "|")
}

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
	Namespace string // Stable route identity: IFD path, namespace URI, or box path family
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
	// Zero value means all primary sources plus CONFIG.
	Sources Source

	// HandleTag is called for each decoded tag. Required for Decode().
	HandleTag HandleTagFunc

	// ShouldHandleTag filters tags before HandleTag. Return false to skip.
	ShouldHandleTag func(TagInfo) bool

	// Warnf receives non-fatal warnings (e.g., skipped boxes, partial decodes).
	Warnf func(format string, args ...any)

	// Timeout bounds total decode time. Zero means no timeout.
	// Warning: when a timeout fires, DecodeResult may contain partial data
	// because the decode goroutine continues running briefly after cancellation.
	Timeout time.Duration

	// LimitNumTags caps total tags delivered. Default 5000.
	LimitNumTags uint32

	// LimitTagSize caps individual tag value size in bytes. Zero means no limit.
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

// Metadata is returned by DecodeAll.
type Metadata struct {
	Tags        Tags
	VideoConfig VideoConfig
}

// ErrStopWalking can be returned from HandleTag to stop decoding early.
var ErrStopWalking = errors.New("stop walking")

// Decode reads metadata from a video file, calling opts.HandleTag for each tag.
func Decode(opts Options) (result DecodeResult, err error) {
	var wantsConfig bool

	defer func() {
		if !wantsConfig {
			result.VideoConfig = VideoConfig{}
		}
	}()

	if opts.R == nil {
		return result, fmt.Errorf("videometa: Options.R is required")
	}
	if opts.HandleTag == nil {
		return result, fmt.Errorf("videometa: Options.HandleTag is required")
	}
	if opts.Sources.IsZero() {
		opts.Sources = QUICKTIME | VENDOR | CONFIG
	}
	wantsConfig = opts.Sources.Has(CONFIG)
	opts.Sources = opts.Sources.Remove(COMPOSITE)
	if opts.Sources.IsZero() {
		return result, nil
	}
	if opts.LimitNumTags == 0 {
		opts.LimitNumTags = 5000
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

type sourceLookupKey struct {
	namespace string
	tag       string
}

type namespaceTagCollection struct {
	ordered []TagInfo
	byTag   map[string][]TagInfo
}

func newNamespaceTagCollection() *namespaceTagCollection {
	return &namespaceTagCollection{
		byTag: make(map[string][]TagInfo),
	}
}

func (c *namespaceTagCollection) add(tag TagInfo) {
	c.ordered = append(c.ordered, tag)
	c.byTag[tag.Tag] = append(c.byTag[tag.Tag], tag)
}

type sourceTagCollection struct {
	ordered        []TagInfo
	namespaceOrder []string
	namespaces     map[string]*namespaceTagCollection
	byTag          map[string][]TagInfo
	byNamespaceTag map[sourceLookupKey][]TagInfo
}

func newSourceTagCollection() *sourceTagCollection {
	return &sourceTagCollection{
		namespaces:     make(map[string]*namespaceTagCollection),
		byTag:          make(map[string][]TagInfo),
		byNamespaceTag: make(map[sourceLookupKey][]TagInfo),
	}
}

func (c *sourceTagCollection) add(tag TagInfo) {
	c.ordered = append(c.ordered, tag)
	namespaceTags, found := c.namespaces[tag.Namespace]
	if !found {
		namespaceTags = newNamespaceTagCollection()
		c.namespaces[tag.Namespace] = namespaceTags
		c.namespaceOrder = append(c.namespaceOrder, tag.Namespace)
	}
	namespaceTags.add(tag)
	c.byTag[tag.Tag] = append(c.byTag[tag.Tag], tag)
	key := sourceLookupKey{namespace: tag.Namespace, tag: tag.Tag}
	c.byNamespaceTag[key] = append(c.byNamespaceTag[key], tag)
}

// Tags collects decoded metadata for convenient access via DecodeAll.
type Tags struct {
	ordered []TagInfo
	sources map[Source]*sourceTagCollection
}

// SourceTags exposes lossless, namespace-aware access to tags from one source.
type SourceTags struct {
	collection *sourceTagCollection
}

// NamespaceTags exposes lossless access to tags from one namespace.
type NamespaceTags struct {
	collection *namespaceTagCollection
}

// Add stores a tag while preserving source, namespace, tag, and decode order.
func (t *Tags) Add(tag TagInfo) {
	if tag.Source == CONFIG {
		return
	}
	if t.sources == nil {
		t.sources = make(map[Source]*sourceTagCollection)
	}
	collection := t.sources[tag.Source]
	if collection == nil {
		collection = newSourceTagCollection()
		t.sources[tag.Source] = collection
	}
	t.ordered = append(t.ordered, tag)
	collection.add(tag)
}

func (t Tags) source(source Source) SourceTags {
	if t.sources == nil {
		return SourceTags{}
	}
	return SourceTags{collection: t.sources[source]}
}

// All returns all tags in decode order, including duplicates across namespaces.
func (t Tags) All() []TagInfo {
	return slices.Clone(t.ordered)
}

// QuickTime returns all standard QuickTime tags.
func (t Tags) QuickTime() SourceTags { return t.source(QUICKTIME) }

// Vendor returns all vendor-specific container tags.
func (t Tags) Vendor() SourceTags { return t.source(VENDOR) }

// Composite returns all derived tags.
func (t Tags) Composite() SourceTags { return t.source(COMPOSITE) }

// All returns all tags for this source in decode order.
func (s SourceTags) All() []TagInfo {
	if s.collection == nil {
		return nil
	}
	return slices.Clone(s.collection.ordered)
}

// Namespaces returns namespaces for this source in first-seen order.
func (s SourceTags) Namespaces() []string {
	if s.collection == nil {
		return nil
	}
	return slices.Clone(s.collection.namespaceOrder)
}

// Namespace returns a lossless subview over one namespace.
func (s SourceTags) Namespace(name string) NamespaceTags {
	if s.collection == nil {
		return NamespaceTags{}
	}
	namespaceTags, found := s.collection.namespaces[name]
	if !found {
		return NamespaceTags{}
	}
	return NamespaceTags{collection: namespaceTags}
}

// FindInNamespace finds all tags by exact namespace and tag name in decode order.
func (s SourceTags) FindInNamespace(namespace, tag string) []TagInfo {
	if s.collection == nil {
		return nil
	}
	return slices.Clone(s.collection.byNamespaceTag[sourceLookupKey{namespace: namespace, tag: tag}])
}

// Find finds all tags with the given tag name across namespaces in decode order.
func (s SourceTags) Find(tag string) []TagInfo {
	if s.collection == nil {
		return nil
	}
	return slices.Clone(s.collection.byTag[tag])
}

// All returns all tags in this namespace in decode order.
func (n NamespaceTags) All() []TagInfo {
	if n.collection == nil {
		return nil
	}
	return slices.Clone(n.collection.ordered)
}

// Find finds all tags with the given name in this namespace in decode order.
func (n NamespaceTags) Find(tag string) []TagInfo {
	if n.collection == nil {
		return nil
	}
	return slices.Clone(n.collection.byTag[tag])
}

func firstTagInfo(sourceTags SourceTags, keys ...string) (TagInfo, bool) {
	for _, key := range keys {
		if matches := sourceTags.Find(key); len(matches) > 0 {
			return matches[0], true
		}
	}
	return TagInfo{}, false
}

func firstTagValue(sourceTags SourceTags, keys ...string) (any, bool) {
	if tag, found := firstTagInfo(sourceTags, keys...); found {
		return tag.Value, true
	}
	return nil, false
}

func firstNumericValue(sources []SourceTags, keys ...string) (float64, bool) {
	for _, sourceTags := range sources {
		if tag, found := firstTagInfo(sourceTags, keys...); found {
			if value, ok := toFloat64(tag.Value); ok {
				return value, true
			}
		}
	}
	return 0, false
}

// GetDateTime returns the best available creation time with original timezone.
// Priority: QuickTime CreationDate/CreateDate/ModifyDate >
// vendor-specific container metadata creation tags.
func (t Tags) GetDateTime() (time.Time, error) {
	candidates := []struct {
		source SourceTags
		keys   []string
	}{
		{source: t.QuickTime(), keys: []string{"CreationDate", "CreateDate", "ModifyDate"}},
		{source: t.Vendor(), keys: []string{"CreationDate", "CreateDate", "ModifyDate", "DateTimeOriginal"}},
	}

	for _, candidate := range candidates {
		if tag, found := firstTagInfo(candidate.source, candidate.keys...); found {
			if dt, err := parseAnyDateTime(tag.Value); err == nil {
				return dt, nil
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
// Priority: QuickTime GPS > vendor-specific container GPS.
func (t Tags) GetLatLong() (lat, lon float64, err error) {
	// Try QuickTime GPS (space-separated decimal or ISO6709).
	if gpsTag, found := firstTagInfo(t.QuickTime(), "GPSCoordinates"); found {
		if s, ok := gpsTag.Value.(string); ok {
			if lat, lon, err := parseGPSCoordinatesString(s); err == nil {
				return lat, lon, nil
			}
		}
	}

	if latTag, found := firstTagInfo(t.Vendor(), "GPSLatitude", "Latitude"); found {
		if lonTag, found := firstTagInfo(t.Vendor(), "GPSLongitude", "Longitude"); found {
			if latVal, ok := toFloat64(latTag.Value); ok {
				if lonVal, ok := toFloat64(lonTag.Value); ok {
					return latVal, lonVal, nil
				}
			}
		}
	}

	return 0, 0, fmt.Errorf("videometa: no GPS coordinates found")
}

// DecodeAll collects emitted tags and returns them together with VideoConfig.
func DecodeAll(opts Options) (Metadata, error) {
	var metadata Metadata
	requestedSources := opts.Sources
	if requestedSources.IsZero() {
		requestedSources = QUICKTIME | VENDOR | CONFIG | COMPOSITE
	}

	decodeSources := requestedSources.Remove(COMPOSITE)
	if requestedSources.Has(COMPOSITE) {
		decodeSources |= QUICKTIME | VENDOR | CONFIG
	}
	opts.Sources = decodeSources
	opts.HandleTag = func(ti TagInfo) error {
		metadata.Tags.Add(ti)
		return nil
	}
	result, err := Decode(opts)
	metadata.VideoConfig = result.VideoConfig
	if requestedSources.Has(COMPOSITE) {
		computeComposite(&metadata.Tags, metadata.VideoConfig)
	}
	return metadata, err
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
	if ti.Source == COMPOSITE || ti.Source == CONFIG {
		return
	}
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

// computeComposite derives Composite tags from already-decoded data, matching
// exiftool's Composite group output.
func computeComposite(tags *Tags, config VideoConfig) {
	add := func(name string, value any) {
		tags.Add(TagInfo{Source: COMPOSITE, Tag: name, Namespace: "Composite", Value: value})
	}

	w := config.Width
	h := config.Height

	// Photography composites follow exiftool's Composite ordering before the
	// generic image-size and bitrate fields.
	if value, found := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "FNumber"); found {
		add("Aperture", value)
	}

	if w > 0 && h > 0 {
		add("ImageSize", fmt.Sprintf("%d %d", w, h))
		add("Megapixels", float64(w*h)/1000000.0)
	}

	if value, found := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "ExposureTime"); found {
		add("ShutterSpeed", value)
	}

	// AvgBitrate: MediaDataSize * 8 / Duration.
	if mdSize, found := firstTagInfo(tags.QuickTime(), "MediaDataSize"); found {
		if dur, found := firstTagInfo(tags.QuickTime(), "Duration"); found {
			if sizeF, ok := toFloat64(mdSize.Value); ok {
				if durF, ok := toFloat64(dur.Value); ok && durF > 0 {
					add("AvgBitrate", int(math.Round(sizeF*8/durF)))
				}
			}
		}
	}

	// GPS decomposition from QuickTime GPSCoordinates (space-separated decimal).
	if gpsTag, found := firstTagInfo(tags.QuickTime(), "GPSCoordinates"); found {
		if s, ok := gpsTag.Value.(string); ok {
			lat, lon, err := parseGPSCoordinatesString(s)
			alt, altOK := parseGPSAltitudeFromString(s)
			if altOK {
				ref := 0
				if alt < 0 {
					ref = 1
					alt = -alt
				}
				add("GPSAltitude", alt)
				add("GPSAltitudeRef", ref)
			}
			if err == nil {
				add("GPSLatitude", lat)
				add("GPSLongitude", lon)
			}
			add("Rotation", config.Rotation)
			if err == nil {
				add("GPSPosition", fmt.Sprintf("%g %g", lat, lon))
			}
		} else {
			add("Rotation", config.Rotation)
		}
	} else {
		add("Rotation", config.Rotation)
	}

	if value, found := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "FocalLength"); found {
		add("FocalLength35efl", value)
	}

	// LightValue: log2(Aperture^2 / ShutterSpeed) - log2(ISO/100).
	ap, apOK := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "FNumber")
	et, etOK := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "ExposureTime")
	iso, isoOK := firstNumericValue([]SourceTags{tags.Vendor(), tags.QuickTime()}, "ISO")
	if apOK && etOK && isoOK && ap > 0 && et > 0 && iso > 0 {
		lv := math.Log2(ap*ap/et) - math.Log2(iso/100)
		add("LightValue", lv)
	}

	// LensID from vendor or QuickTime lens metadata.
	if lensID, found := firstTagValue(tags.Vendor(), "LensModel"); found {
		add("LensID", lensID)
		return
	}
	if lensID, found := firstTagValue(tags.QuickTime(), "LensModel"); found {
		add("LensID", lensID)
		return
	}
	if lensID, found := firstTagValue(tags.Vendor(), "LensID"); found {
		add("LensID", lensID)
		return
	}
	if lensID, found := firstTagValue(tags.QuickTime(), "LensID"); found {
		add("LensID", lensID)
	}
}
