package videometa

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// iTunes/QuickTime data type indicators (well-known types).
const (
	qtDataTypeUTF8       = 1
	qtDataTypeUTF16BE    = 2
	qtDataTypeSJIS       = 3
	qtDataTypeHTML       = 6
	qtDataTypeXML        = 7
	qtDataTypeUUID       = 8
	qtDataTypeISRC       = 9
	qtDataTypeBMP        = 14
	qtDataTypeJPEG       = 13
	qtDataTypePNG        = 14
	qtDataTypeSInt8      = 21
	qtDataTypeUInt8      = 22
	qtDataTypeSInt16BE   = 23
	qtDataTypeUInt16BE   = 24
	qtDataTypeSInt32BE   = 25
	qtDataTypeUInt32BE   = 26
	qtDataTypeSInt64BE   = 27
	qtDataTypeUInt64BE   = 28
	qtDataTypeFloat32BE  = 29
	qtDataTypeFloat64BE  = 30
)

// decodeIlst parses the ilst (item list) box containing QuickTime metadata atoms.
func (d *videoDecoderMP4) decodeIlst(ilstStart int64, ilstSize uint64) {
	ilstEnd := ilstStart + int64(ilstSize)
	for d.pos() < ilstEnd {
		atomStart := d.pos()
		atomSize, atomType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		atomTypeStr := atomType.String()
		atomEnd := atomStart + int64(atomSize)

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
	atomEnd := atomStart + int64(atomSize)

	for d.pos() < atomEnd {
		dataStart := d.pos()
		dataSize, dataType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		if dataType.String() == "data" {
			typeIndicator := d.read4()
			_ = d.read4() // locale

			valueLen := int(dataSize) - 16 // 8 (box header) + 4 (type) + 4 (locale)
			if valueLen <= 0 {
				break
			}

			value := d.decodeQTValue(typeIndicator, valueLen)
			if value != nil {
				d.emitQuickTimeTag(tagName, value)
			}
		}

		dataEnd := dataStart + int64(dataSize)
		if d.pos() < dataEnd {
			d.skip(dataEnd - d.pos())
		}
	}
}

// decodeFreeformAtom parses a freeform (----) atom with mean, name, and data sub-atoms.
func (d *videoDecoderMP4) decodeFreeformAtom(atomStart int64, atomSize uint64) {
	atomEnd := atomStart + int64(atomSize)
	var mean, name string

	for d.pos() < atomEnd {
		subStart := d.pos()
		subSize, subType, isEOF := d.readBoxHeader()
		if isEOF {
			break
		}

		subEnd := subStart + int64(subSize)
		subTypeStr := subType.String()

		switch subTypeStr {
		case "mean":
			_ = d.readBytes(4) // version + flags
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
func (d *videoDecoderMP4) decodeQTValue(typeIndicator uint32, valueLen int) any {
	switch typeIndicator {
	case qtDataTypeUTF8:
		return printableString(string(d.readBytes(valueLen)))
	case qtDataTypeUTF16BE:
		data := d.readBytes(valueLen)
		return decodeUTF16BE(data)
	case qtDataTypeSInt8:
		if valueLen >= 1 {
			return int8(d.read1())
		}
	case qtDataTypeUInt8:
		if valueLen >= 1 {
			return d.read1()
		}
	case qtDataTypeSInt16BE:
		if valueLen >= 2 {
			return int16(d.read2())
		}
	case qtDataTypeUInt16BE:
		if valueLen >= 2 {
			return d.read2()
		}
	case qtDataTypeSInt32BE:
		if valueLen >= 4 {
			return d.read4s()
		}
	case qtDataTypeUInt32BE:
		if valueLen >= 4 {
			return d.read4()
		}
	case qtDataTypeSInt64BE:
		if valueLen >= 8 {
			return int64(d.read8())
		}
	case qtDataTypeUInt64BE:
		if valueLen >= 8 {
			return d.read8()
		}
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

// parseQuickTimeDate parses a QuickTime date string like "2024-06-15T10:30:00+0200".
func parseQuickTimeDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
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
	"make":               "Make",
	"model":              "Model",
	"software":           "Software",
	"creationdate":       "CreationDate",
	"location.ISO6709":   "GPSCoordinates",
	"location.role":      "LocationRole",
	"location.body":      "LocationBody",
	"location.note":      "LocationNote",
	"camera.identifier":  "CameraIdentifier",
	"camera.framereadouttimeinmicroseconds": "CameraFrameReadoutTime",
	"player.version":     "PlayerVersion",
	"player.movie.visual.brightness":    "Brightness",
	"player.movie.visual.contrast":      "Contrast",
	"player.movie.audio.gain":           "AudioGain",
	"player.movie.audio.treble":         "AudioTreble",
	"player.movie.audio.bass":           "AudioBass",
	"player.movie.audio.balance":        "AudioBalance",
	"player.movie.audio.pitchshift":     "PitchShift",
	"player.movie.audio.mute":           "Mute",
	"live-photo.auto":                   "LivePhotoAuto",
	"live-photo.vitality-score":         "LivePhotoVitalityScore",
	"live-photo.vitality-scoring-version": "LivePhotoVitalityScoringVersion",
	"content.identifier": "ContentIdentifier",
	"detected-face.count": "DetectedFaceCount",
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
	"\xa9nam": "Title",
	"\xa9ART": "Artist",
	"\xa9alb": "Album",
	"\xa9day": "ContentCreateDate",
	"\xa9too": "Encoder",
	"\xa9cmt": "Comment",
	"\xa9gen": "Genre",
	"\xa9wrt": "Composer",
	"\xa9grp": "Grouping",
	"\xa9lyr": "Lyrics",
	"\xa9des": "Description",
	"\xa9enc": "EncodedBy",
	"\xa9dir": "Director",
	"\xa9prd": "Producer",
	"\xa9prf": "Performers",
	"\xa9inf": "Information",
	"\xa9req": "Requirements",
	"\xa9fmt": "Format",
	"\xa9src": "Source",
	"\xa9swr": "SoftwareVersion",
	"\xa9xyz": "GPSCoordinates",
	"aART":    "AlbumArtist",
	"trkn":    "TrackNumber",
	"disk":    "DiskNumber",
	"tmpo":    "BeatsPerMinute",
	"cpil":    "Compilation",
	"covr":    "CoverArt",
	"pgap":    "PlayGap",
	"gnre":    "GenreID",
	"cprt":    "Copyright",
	"desc":    "Description",
	"ldes":    "LongDescription",
	"catg":    "Category",
	"keyw":    "Keyword",
	"purd":    "PurchaseDate",
	"pcst":    "Podcast",
	"purl":    "PodcastURL",
	"hdvd":    "HDVideo",
	"stik":    "MediaType",
	"rtng":    "Rating",
	"apID":    "AppleStoreAccount",
	"sfID":    "AppleStoreCountry",
	"akID":    "AppleStoreAccountType",
	"cnID":    "AppleStoreCatalogID",
	"geID":    "GenreID",
	"atID":    "ArtistID",
	"plID":    "PlaylistID",
	"cmID":    "ComposerID",
	"sonm":    "SortName",
	"soar":    "SortArtist",
	"soal":    "SortAlbum",
	"soco":    "SortComposer",
	"sosn":    "SortShow",
	"tvsh":    "TVShow",
	"tvsn":    "TVSeason",
	"tves":    "TVEpisode",
	"tvnn":    "TVNetwork",
	"pcsn":    "SortPodcast",
	strings.Repeat("\x00", 4): "", // Null atom — skip.
}
