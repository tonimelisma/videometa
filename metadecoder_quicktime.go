package videometa

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// iTunes/QuickTime data type indicators (well-known types).
const (
	qtDataTypeUTF8      = 1
	qtDataTypeUTF16BE   = 2
	qtDataTypeSJIS      = 3
	qtDataTypeHTML      = 6
	qtDataTypeXML       = 7
	qtDataTypeUUID      = 8
	qtDataTypeISRC      = 9
	qtDataTypeBMP       = 14
	qtDataTypeJPEG      = 13
	qtDataTypePNG       = 14
	qtDataTypeSInt8     = 21
	qtDataTypeUInt8     = 22
	qtDataTypeSInt16BE  = 23
	qtDataTypeUInt16BE  = 24
	qtDataTypeSInt32BE  = 25
	qtDataTypeUInt32BE  = 26
	qtDataTypeSInt64BE  = 27
	qtDataTypeUInt64BE  = 28
	qtDataTypeFloat32BE = 29
	qtDataTypeFloat64BE = 30
)

// decodeIlst parses the ilst (item list) box containing QuickTime metadata atoms.
func (d *videoDecoderMP4) decodeIlst(ilstStart int64, ilstSize uint64) {
	ilstEnd := boxEnd(ilstStart, ilstSize)
	for d.pos() < ilstEnd {
		atomStart := d.pos()
		atomSize, atomType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		atomTypeStr := atomType.String()
		atomEnd := boxEnd(atomStart, atomSize)

		if atomTypeStr == "----" {
			// Freeform atom: mean + name + data sub-atoms.
			d.decodeFreeformAtom(atomStart, atomSize)
		} else {
			// Standard ilst atom: look up tag name and parse data.
			tagName := ilstAtomToTagName(atomTypeStr)
			if tagName != "" {
				d.decodeIlstAtomData(atomStart, atomSize, tagName)
			}
		}

		// Seek past atom regardless of whether we parsed it.
		if d.pos() < atomEnd {
			d.skip(atomEnd - d.pos())
		}
	}
}

// decodeIlstAtomData parses the data sub-box of a standard ilst atom.
func (d *videoDecoderMP4) decodeIlstAtomData(atomStart int64, atomSize uint64, tagName string) {
	atomEnd := boxEnd(atomStart, atomSize)

	for d.pos() < atomEnd {
		dataStart := d.pos()
		dataSize, dataType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		if dataType.String() == "data" { //nolint:gocritic
			typeIndicator := d.read4()
			locale := d.read4()

			valueLen := int(dataSize) - 16 // 8 (box header) + 4 (type) + 4 (locale)
			if valueLen <= 0 {
				break
			}

			// Special handling for binary atoms with non-standard encoding.
			var value any
			switch {
			case (tagName == "TrackNumber" || tagName == "DiskNumber") && typeIndicator == 0 && valueLen >= 6:
				value = d.decodeTrackDiskNumber(valueLen)
			case tagName == "BeatsPerMinute" && typeIndicator == qtDataTypeSInt8 && valueLen >= 2:
				// tmpo stores BPM as big-endian uint16, but type indicator may incorrectly say int8.
				value = int(binary.BigEndian.Uint16(d.readBytes(valueLen)[:2]))
			default:
				value = d.decodeQTValue(typeIndicator, valueLen)
			}
			if value != nil {
				if locale != 0 {
					// Emit localized variant with language-country suffix,
					// plus a synthesized default-language (unsuffixed) tag
					// matching exiftool behavior.
					suffix := decodeLocale(locale)
					if suffix != "" {
						d.emitQuickTimeTag(tagName+"-"+suffix, value)
					}
					d.emitQuickTimeTag(tagName, value)
				} else {
					d.emitQuickTimeTag(tagName, value)
				}
			}
		}

		dataEnd := boxEnd(dataStart, dataSize)
		if d.pos() < dataEnd {
			d.skip(dataEnd - d.pos())
		}
	}
}

// decodeFreeformAtom parses a freeform (----) atom with mean, name, and data sub-atoms.
func (d *videoDecoderMP4) decodeFreeformAtom(atomStart int64, atomSize uint64) {
	atomEnd := boxEnd(atomStart, atomSize)
	var mean, name string

	for d.pos() < atomEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		subEnd := boxEnd(subStart, subSize)
		subTypeStr := subType.String()

		switch subTypeStr {
		case "mean":
			_ = d.readBytes(4)            // version + flags
			valueLen := int(subSize) - 12 // 8 (header) + 4 (version+flags)
			if valueLen > 0 {
				mean = string(d.readBytes(valueLen))
			}
		case "name":
			_ = d.readBytes(4) // version + flags
			valueLen := int(subSize) - 12
			if valueLen > 0 {
				name = string(d.readBytes(valueLen))
			}
		case "data":
			if mean != "" && name != "" {
				typeIndicator := d.read4()
				_ = d.read4() // locale
				valueLen := int(subSize) - 16
				if valueLen > 0 {
					value := d.decodeQTValue(typeIndicator, valueLen)
					if value != nil {
						tagName := freeformToTagName(mean, name)
						if tagName != "" {
							d.emitQuickTimeTag(tagName, value)
						}
					}
				}
			}
		}

		if d.pos() < subEnd {
			d.skip(subEnd - d.pos())
		}
	}
}

// decodeQTValue decodes a QuickTime data value based on its type indicator.
// Apple stores integer values in padded big-endian slots (e.g., int8 in 8 bytes),
// so we read the full valueLen as a big-endian integer for all integer types.
func (d *videoDecoderMP4) decodeQTValue(typeIndicator uint32, valueLen int) any {
	switch typeIndicator {
	case qtDataTypeUTF8:
		// Preserve trailing spaces (exiftool does), but strip nulls and control chars.
		return cleanQTString(string(d.readBytes(valueLen)))
	case qtDataTypeUTF16BE:
		data := d.readBytes(valueLen)
		return decodeUTF16BE(data)
	case qtDataTypeSInt8, qtDataTypeSInt16BE, qtDataTypeSInt32BE, qtDataTypeSInt64BE:
		if valueLen < 1 {
			return nil
		}
		return int64(readBEIntN(d.readBytes(valueLen)))
	case qtDataTypeUInt8, qtDataTypeUInt16BE, qtDataTypeUInt32BE, qtDataTypeUInt64BE:
		if valueLen < 1 {
			return nil
		}
		return readBEUintN(d.readBytes(valueLen))
	case qtDataTypeFloat32BE:
		if valueLen >= 4 {
			bits := d.read4()
			return float64(math.Float32frombits(bits))
		}
	case qtDataTypeFloat64BE:
		if valueLen >= 8 {
			bits := d.read8()
			return math.Float64frombits(bits)
		}
	default:
		// Unknown type — return as raw bytes or skip.
		if valueLen > 0 {
			return string(trimNulls(d.readBytes(valueLen)))
		}
	}
	return nil
}

// readBEUintN reads up to 8 bytes as a big-endian unsigned integer.
func readBEUintN(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

// readBEIntN reads up to 8 bytes as a big-endian signed integer (sign-extended).
func readBEIntN(b []byte) int64 {
	if len(b) == 0 {
		return 0
	}
	// Sign-extend from the MSB.
	var v int64
	if b[0]&0x80 != 0 {
		v = -1 // All ones for sign extension.
	}
	for _, c := range b {
		v = v<<8 | int64(c)
	}
	return v
}

// cleanQTString removes nulls and control characters but preserves spaces.
func cleanQTString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == 0 {
			continue
		}
		if r >= 32 || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// decodeTrackDiskNumber decodes the binary trkn/disk atom value.
// Format: 2 bytes padding + uint16 number + uint16 total [+ 2 padding].
// Returns "N of M" matching exiftool.
func (d *videoDecoderMP4) decodeTrackDiskNumber(valueLen int) any {
	data := d.readBytes(valueLen)
	if len(data) < 6 {
		return nil
	}
	num := binary.BigEndian.Uint16(data[2:4])
	total := binary.BigEndian.Uint16(data[4:6])
	if total > 0 {
		return fmt.Sprintf("%d of %d", num, total)
	}
	return fmt.Sprintf("%d", num)
}

// decodeUTF16BE decodes a UTF-16 big-endian byte slice to a Go string.
func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		r := rune(binary.BigEndian.Uint16(data[i:]))
		if r == 0 {
			break
		}
		runes = append(runes, r)
	}
	return string(runes)
}

// decodeLocale converts a QuickTime locale uint32 to a language-country string
// like "eng-US". The format is: country(16 bits, ASCII) + language(16 bits, packed ISO-639).
func decodeLocale(locale uint32) string {
	if locale == 0 {
		return ""
	}

	// Upper 16 bits: country code as 2 ASCII bytes.
	countryHi := byte(locale >> 24)
	countryLo := byte(locale >> 16)

	// Lower 16 bits: packed ISO-639-2/T language (5 bits per char).
	packed := uint16(locale & 0xFFFF)
	c1 := byte((packed>>10)&0x1F) + 0x60
	c2 := byte((packed>>5)&0x1F) + 0x60
	c3 := byte(packed&0x1F) + 0x60
	lang := string([]byte{c1, c2, c3})

	if lang == "\x60\x60\x60" || lang == "und" {
		return ""
	}

	// Append country if present and printable.
	if countryHi >= 'A' && countryHi <= 'Z' && countryLo >= 'A' && countryLo <= 'Z' {
		return lang + "-" + string([]byte{countryHi, countryLo})
	}
	return lang
}

// freeformToTagName maps com.apple.quicktime freeform atoms to exiftool tag names.
func freeformToTagName(mean, name string) string {
	if mean != "com.apple.quicktime" {
		// For other vendors, construct a tag name.
		return ""
	}
	if tagName, ok := freeformTagNames[name]; ok {
		return tagName
	}
	return ""
}

// freeformTagNames maps com.apple.quicktime key names to exiftool tag names.
var freeformTagNames = map[string]string{
	"make":                                  "Make",
	"model":                                 "Model",
	"software":                              "Software",
	"creationdate":                          "CreationDate",
	"location.ISO6709":                      "GPSCoordinates",
	"location.role":                         "LocationRole",
	"location.body":                         "LocationBody",
	"location.note":                         "LocationNote",
	"camera.identifier":                     "CameraIdentifier",
	"camera.framereadouttimeinmicroseconds": "CameraFrameReadoutTime",
	"player.version":                        "PlayerVersion",
	"player.movie.visual.brightness":        "Brightness",
	"player.movie.visual.contrast":          "Contrast",
	"player.movie.audio.gain":               "AudioGain",
	"player.movie.audio.treble":             "AudioTreble",
	"player.movie.audio.bass":               "AudioBass",
	"player.movie.audio.balance":            "AudioBalance",
	"player.movie.audio.pitchshift":         "PitchShift",
	"player.movie.audio.mute":               "Mute",
	"live-photo.auto":                       "LivePhotoAuto",
	"live-photo.vitality-score":             "LivePhotoVitalityScore",
	"live-photo.vitality-scoring-version":   "LivePhotoVitalityScoringVersion",
	"content.identifier":                    "ContentIdentifier",
	"detected-face.count":                   "DetectedFaceCount",
	"camera.lens_model":                     "LensModel",
	"camera.focal_length.35mm_equivalent":   "FocalLengthIn35mmFormat",
	"camera.lens_irisfnumber":               "CameraLensIrisfnumber",
	"location.accuracy.horizontal":          "LocationAccuracyHorizontal",
	"full-frame-rate-playback-intent":       "FullFrameRatePlaybackIntent",
	"apple-maker-note.74":                   "Apple-maker-note74",
	"apple-maker-note.97":                   "Apple-maker-note97",
}

// ilstAtomToTagName maps standard ilst atom types to exiftool tag names.
func ilstAtomToTagName(atomType string) string {
	if name, ok := ilstTagNames[atomType]; ok {
		return name
	}
	return ""
}

// Standard ilst atom type to exiftool tag name mapping.
var ilstTagNames = map[string]string{
	"\xa9nam":                 "Title",
	"\xa9ART":                 "Artist",
	"\xa9alb":                 "Album",
	"\xa9day":                 "ContentCreateDate",
	"\xa9too":                 "Encoder",
	"\xa9cmt":                 "Comment",
	"\xa9gen":                 "Genre",
	"\xa9wrt":                 "Composer",
	"\xa9grp":                 "Grouping",
	"\xa9lyr":                 "Lyrics",
	"\xa9des":                 "Description",
	"\xa9enc":                 "EncodedBy",
	"\xa9dir":                 "Director",
	"\xa9prd":                 "Producer",
	"\xa9prf":                 "Performers",
	"\xa9inf":                 "Information",
	"\xa9req":                 "Requirements",
	"\xa9fmt":                 "Format",
	"\xa9src":                 "Source",
	"\xa9swr":                 "SoftwareVersion",
	"\xa9xyz":                 "GPSCoordinates",
	"aART":                    "AlbumArtist",
	"trkn":                    "TrackNumber",
	"disk":                    "DiskNumber",
	"tmpo":                    "BeatsPerMinute",
	"cpil":                    "Compilation",
	"covr":                    "CoverArt",
	"pgap":                    "PlayGap",
	"gnre":                    "GenreID",
	"cprt":                    "Copyright",
	"desc":                    "Description",
	"ldes":                    "LongDescription",
	"catg":                    "Category",
	"keyw":                    "Keyword",
	"purd":                    "PurchaseDate",
	"pcst":                    "Podcast",
	"purl":                    "PodcastURL",
	"hdvd":                    "HDVideo",
	"stik":                    "MediaType",
	"rtng":                    "Rating",
	"apID":                    "AppleStoreAccount",
	"sfID":                    "AppleStoreCountry",
	"akID":                    "AppleStoreAccountType",
	"cnID":                    "AppleStoreCatalogID",
	"geID":                    "GenreID",
	"atID":                    "ArtistID",
	"plID":                    "PlaylistID",
	"cmID":                    "ComposerID",
	"sonm":                    "SortName",
	"soar":                    "SortArtist",
	"soal":                    "SortAlbum",
	"soco":                    "SortComposer",
	"sosn":                    "SortShow",
	"tvsh":                    "TVShow",
	"tvsn":                    "TVSeason",
	"tves":                    "TVEpisode",
	"tvnn":                    "TVNetwork",
	"pcsn":                    "SortPodcast",
	strings.Repeat("\x00", 4): "", // Null atom — skip.
}
