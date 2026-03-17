package videometa

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// XMP UUID used to locate XMP data in ISOBMFF containers.
var xmpUUID = [16]byte{
	0xBE, 0x7A, 0xCF, 0xCB, 0x97, 0xA9, 0x42, 0xE8,
	0x9C, 0x71, 0x99, 0x94, 0x91, 0xE3, 0xAF, 0xAC,
}

// decodeXMP parses XMP/RDF XML from the given reader.
func (d *videoDecoderMP4) decodeXMP(r io.Reader) error {
	if d.opts.HandleXMP != nil {
		return d.opts.HandleXMP(r)
	}

	var meta xmpmeta
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&meta); err != nil {
		if d.opts.Warnf != nil {
			d.opts.Warnf("decode xmp: %v", err)
		}
		return nil // Non-fatal — partial failure.
	}

	for _, desc := range meta.RDF.Descriptions {
		// Process attributes on the Description element.
		for _, attr := range desc.Attrs {
			if isXMPMetaNamespace(attr.Name.Space) {
				continue
			}
			if attr.Value == "" {
				continue
			}
			d.emitXMPTag(attr.Name.Local, attr.Name.Space, attr.Value)
		}

		// Process list elements.
		d.emitXMPList("Creator", desc.Creator)
		d.emitXMPList("Subject", desc.Subject)
		d.emitXMPList("Publisher", desc.Publisher)
		d.emitXMPList("Rights", desc.Rights)
		d.emitXMPList("Title", desc.Title)
		d.emitXMPList("Description", desc.Description)

		// Process GPS child elements.
		if desc.GPSLatitude != "" {
			if lat, err := parseXMPGPSCoordinate(desc.GPSLatitude); err == nil {
				d.emitXMPTag("GPSLatitude", "http://ns.adobe.com/exif/1.0/", lat)
			}
		}
		if desc.GPSLongitude != "" {
			if lon, err := parseXMPGPSCoordinate(desc.GPSLongitude); err == nil {
				d.emitXMPTag("GPSLongitude", "http://ns.adobe.com/exif/1.0/", lon)
			}
		}
	}

	return nil
}

// XMP XML structures matching the RDF format.
type xmpmeta struct {
	XMLName xml.Name `xml:"xmpmeta"`
	RDF     rdf      `xml:"RDF"`
}

type rdf struct {
	XMLName      xml.Name
	Descriptions []rdfDescription `xml:"Description"`
}

type rdfDescription struct {
	Attrs        []xml.Attr `xml:",any,attr"`
	Creator      seqList    `xml:"creator"`
	Subject      bagList    `xml:"subject"`
	Publisher    bagList    `xml:"publisher"`
	Rights       altList    `xml:"rights"`
	Title        altList    `xml:"title"`
	Description  altList    `xml:"description"`
	GPSLatitude  string     `xml:"GPSLatitude"`
	GPSLongitude string     `xml:"GPSLongitude"`
}

// seqList represents an RDF Seq (ordered list).
type seqList struct {
	Items []string `xml:"Seq>li"`
}

// bagList represents an RDF Bag (unordered list).
type bagList struct {
	Items []string `xml:"Bag>li"`
}

// altList represents an RDF Alt (alternative list).
type altList struct {
	Items []string `xml:"Alt>li"`
}

// isXMPMetaNamespace returns true for XMP/RDF infrastructure namespaces.
func isXMPMetaNamespace(ns string) bool {
	switch ns {
	case "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
		"http://www.w3.org/XML/1998/namespace",
		"xmlns",
		"xml",
		"":
		return true
	}
	return false
}

// emitXMPTag sends an XMP source tag to the callback.
func (d *videoDecoderMP4) emitXMPTag(name, namespace string, value any) {
	if d.opts.HandleTag == nil {
		return
	}
	ti := TagInfo{
		Source:    XMP,
		Tag:       name,
		Namespace: namespace,
		Value:     value,
	}
	if d.opts.ShouldHandleTag != nil && !d.opts.ShouldHandleTag(ti) {
		return
	}
	if err := d.opts.HandleTag(ti); err != nil {
		panic(err)
	}
}

// emitXMPList emits list values — single string for 1 item, []string for multiple.
func (d *videoDecoderMP4) emitXMPList(name string, list interface{ items() []string }) {
	items := list.items()
	if len(items) == 0 {
		return
	}
	if len(items) == 1 {
		d.emitXMPTag(name, "", items[0])
	} else {
		d.emitXMPTag(name, "", items)
	}
}

func (s seqList) items() []string { return s.Items }
func (s bagList) items() []string { return s.Items }
func (s altList) items() []string { return s.Items }

// parseXMPGPSCoordinate parses an XMP GPS coordinate like "26,34.951N" to decimal degrees.
func parseXMPGPSCoordinate(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty GPS coordinate")
	}

	// Determine sign from direction suffix.
	sign := 1.0
	last := s[len(s)-1]
	switch last {
	case 'S', 'W', 's', 'w':
		sign = -1.0
		s = s[:len(s)-1]
	case 'N', 'E', 'n', 'e':
		s = s[:len(s)-1]
	}

	// Try plain decimal.
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return sign * v, nil
	}

	// Try "DD,MM.MMMM" or "DD,MM,SS.SSSS" format.
	parts := strings.Split(s, ",")
	switch len(parts) {
	case 2:
		deg, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("parse degrees: %w", err)
		}
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse minutes: %w", err)
		}
		return sign * (deg + min/60), nil
	case 3:
		deg, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("parse degrees: %w", err)
		}
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse minutes: %w", err)
		}
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, fmt.Errorf("parse seconds: %w", err)
		}
		return sign * (deg + min/60 + sec/3600), nil
	default:
		return 0, fmt.Errorf("unrecognized GPS format: %q", s)
	}
}

// Suppress unused import warning.
var _ = math.NaN
