package videometa

import (
	"io"
	"strings"
)

// IPTC record/dataset marker.
const iptcMarker = 0x1C

// metaDecoderIPTC decodes IPTC-IIM records.
type metaDecoderIPTC struct {
	*streamReader
	opts    Options
	charset string // "UTF-8" or "ISO-8859-1"
}

// decodeIPTC parses IPTC data from the given reader.
func (d *videoDecoderMP4) decodeIPTC(r io.Reader) {
	sr := newStreamReader(r)

	id := &metaDecoderIPTC{
		streamReader: sr,
		opts:         d.opts,
		charset:      "UTF-8", // Default to UTF-8.
	}

	// Recover panics from IPTC decoding — partial failure is OK.
	defer func() {
		if rec := recover(); rec != nil {
			if rec != errStop {
				if d.opts.Warnf != nil {
					d.opts.Warnf("decode iptc: unexpected panic: %v", rec)
				}
			}
		}
	}()

	id.decodeRecords(d)
}

func (id *metaDecoderIPTC) decodeRecords(d *videoDecoderMP4) {
	// Track repeatable fields to accumulate into slices.
	repeatables := make(map[string][]string)
	singles := make(map[string]string)

	// Use panic recovery to handle EOF at end of data.
	defer func() {
		if r := recover(); r != nil {
			if r != errStop {
				panic(r) // Re-panic non-EOF errors.
			}
			// EOF is expected when all records are consumed.
		}

		// Emit collected tags.
		for name, value := range singles {
			d.emitIPTCTag(name, value)
		}
		for name, values := range repeatables {
			if len(values) == 1 {
				d.emitIPTCTag(name, values[0])
			} else {
				d.emitIPTCTag(name, values)
			}
		}
	}()

	for {
		// Read marker.
		marker := id.read1()
		if marker != iptcMarker {
			break // End of IPTC data or invalid.
		}

		recordType := id.read1()
		datasetNum := id.read1()

		// Read data size (2 bytes, big-endian).
		sizeHi := id.read1()
		sizeLo := id.read1()
		size := int(sizeHi)<<8 | int(sizeLo)

		// Extended dataset size (size > 32767).
		if size > 32767 {
			// Not commonly seen in practice — skip.
			break
		}

		if size == 0 {
			continue
		}

		data := id.readBytes(size)

		// Record 1, dataset 90 = coded character set.
		if recordType == 1 && datasetNum == 90 {
			id.detectCharset(data)
			continue
		}

		// Only process Application Record (record 2).
		if recordType != 2 {
			continue
		}

		field := iptcFieldDefs[datasetNum]
		if field.Name == "" {
			continue
		}

		value := id.decodeString(data)

		if field.Repeatable {
			repeatables[field.Name] = append(repeatables[field.Name], value)
		} else {
			singles[field.Name] = value
		}
	}
}

func (id *metaDecoderIPTC) detectCharset(data []byte) {
	// ESC sequences for charset detection.
	// UTF-8: ESC % G (0x1B 0x25 0x47)
	if len(data) >= 3 && data[0] == 0x1B && data[1] == 0x25 && data[2] == 0x47 {
		id.charset = "UTF-8"
		return
	}
	// Default to ISO-8859-1 for other charset indicators.
	id.charset = "ISO-8859-1"
}

func (id *metaDecoderIPTC) decodeString(data []byte) string {
	if id.charset == "ISO-8859-1" {
		return decodeISO88591(data)
	}
	return string(data)
}

// decodeISO88591 converts ISO-8859-1 bytes to a Go UTF-8 string.
func decodeISO88591(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// emitIPTCTag sends an IPTC source tag via the centralized emitTag.
func (d *videoDecoderMP4) emitIPTCTag(name string, value any) {
	d.emitTag(TagInfo{
		Source:    IPTC,
		Tag:       name,
		Namespace: "IPTC",
		Value:     value,
	})
}

// iptcField describes an IPTC Application Record field.
type iptcField struct {
	Name       string
	Repeatable bool
}

// iptcFieldDefs maps Application Record (record 2) dataset numbers to field definitions.
// Names match exiftool output.
var iptcFieldDefs = map[uint8]iptcField{
	0:   {Name: "ApplicationRecordVersion"},
	3:   {Name: "ObjectTypeReference"},
	4:   {Name: "ObjectAttributeReference", Repeatable: true},
	5:   {Name: "ObjectName"},
	7:   {Name: "EditStatus"},
	8:   {Name: "EditorialUpdate"},
	10:  {Name: "Urgency"},
	12:  {Name: "SubjectReference", Repeatable: true},
	15:  {Name: "Category"},
	20:  {Name: "SupplementalCategories", Repeatable: true},
	22:  {Name: "FixtureIdentifier"},
	25:  {Name: "Keywords", Repeatable: true},
	26:  {Name: "ContentLocationCode", Repeatable: true},
	27:  {Name: "ContentLocationName", Repeatable: true},
	30:  {Name: "ReleaseDate"},
	35:  {Name: "ReleaseTime"},
	37:  {Name: "ExpirationDate"},
	38:  {Name: "ExpirationTime"},
	40:  {Name: "SpecialInstructions"},
	42:  {Name: "ActionAdvised"},
	45:  {Name: "ReferenceService", Repeatable: true},
	47:  {Name: "ReferenceDate", Repeatable: true},
	50:  {Name: "ReferenceNumber", Repeatable: true},
	55:  {Name: "DateCreated"},
	60:  {Name: "TimeCreated"},
	62:  {Name: "DigitalCreationDate"},
	63:  {Name: "DigitalCreationTime"},
	65:  {Name: "OriginatingProgram"},
	70:  {Name: "ProgramVersion"},
	75:  {Name: "ObjectCycle"},
	80:  {Name: "By-line", Repeatable: true},
	85:  {Name: "By-lineTitle", Repeatable: true},
	90:  {Name: "City"},
	92:  {Name: "Sub-location"},
	95:  {Name: "Province-State"},
	100: {Name: "Country-PrimaryLocationCode"},
	101: {Name: "Country-PrimaryLocationName"},
	103: {Name: "OriginalTransmissionReference"},
	105: {Name: "Headline"},
	110: {Name: "Credit"},
	115: {Name: "Source"},
	116: {Name: "CopyrightNotice"},
	118: {Name: "Contact", Repeatable: true},
	120: {Name: "Caption-Abstract"},
	122: {Name: "Writer-Editor", Repeatable: true},
	130: {Name: "ImageType"},
	131: {Name: "ImageOrientation"},
	135: {Name: "LanguageIdentifier"},
}
