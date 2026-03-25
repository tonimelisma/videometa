package videometa

import (
	"bytes"
	"encoding/binary"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-EXIF-06
func TestDecodeEXIFFromUUIDBox(t *testing.T) {
	c := qt.New(t)

	data := buildMP4WithEXIFUUID(buildMinimalEXIFASCII(0x010F, "UUID Cam"))

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.EXIF()["Make"].Value, qt.Equals, "UUID Cam")
}

// Validates: REQ-EXIF-06
func TestDecodeEXIFFromMetaIlocFileOffset(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x010F, "Item Offset Cam"))
	data := buildMP4WithMetaFileItem(itemPayload, buildInfeEXIF(1), buildIlocFileOffset(1, 0, uint32(len(itemPayload))))

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.EXIF()["Make"].Value, qt.Equals, "Item Offset Cam")
}

// Validates: REQ-EXIF-06
func TestDecodeEXIFFromMetaIlocIDAT(t *testing.T) {
	c := qt.New(t)

	itemPayload := wrapEXIFItemPayload(buildMinimalEXIFASCII(0x0110, "IDAT Model"))
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeEXIF(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.EXIF()["Model"].Value, qt.Equals, "IDAT Model")
}

// Validates: REQ-XMP-04
func TestDecodeXMPFromMetaIlocIDAT(t *testing.T) {
	c := qt.New(t)

	itemPayload := []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:CreatorTool="Item XMP"/></rdf:RDF></x:xmpmeta>`)
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeXMP(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: XMP,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.XMP()["CreatorTool"].Value, qt.Equals, "Item XMP")
}

func buildMP4WithEXIFUUID(exifPayload []byte) []byte {
	var buf bytes.Buffer
	buf.Write(buildFTYPBox())

	body := make([]byte, 4+len(exifPayload))
	copy(body[4:], exifPayload)

	_ = binary.Write(&buf, binary.BigEndian, uint32(8+16+len(body)))
	buf.WriteString("uuid")
	buf.Write(exifUUID[:])
	buf.Write(body)

	return buf.Bytes()
}

func buildMP4WithMetaFileItem(itemPayload []byte, infeBox []byte, ilocBox []byte) []byte {
	ftyp := buildFTYPBox()
	pitm := buildPITM(1)
	iinf := buildIINF(infeBox)
	meta := buildMetaBox(pitm, iinf, ilocBox)

	payloadBox := buildBox("free", itemPayload)
	payloadOffset := uint32(len(ftyp) + len(meta) + 8)
	meta = buildMetaBox(pitm, iinf, buildIlocFileOffset(1, payloadOffset, uint32(len(itemPayload))))

	return append(append(ftyp, meta...), payloadBox...)
}

func buildMP4WithMetaIDATItem(itemPayload []byte, infeBox []byte, ilocBox []byte) []byte {
	ftyp := buildFTYPBox()
	pitm := buildPITM(1)
	iinf := buildIINF(infeBox)
	idat := buildBox("idat", itemPayload)
	meta := buildMetaBox(pitm, iinf, ilocBox, idat)
	return append(ftyp, meta...)
}

func buildFTYPBox() []byte {
	var payload bytes.Buffer
	payload.WriteString("isom")
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	payload.WriteString("isom")
	return buildBox("ftyp", payload.Bytes())
}

func buildMetaBox(children ...[]byte) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{0, 0, 0, 0}) // version + flags
	for _, child := range children {
		payload.Write(child)
	}
	return buildBox("meta", payload.Bytes())
}

func buildIINF(entries ...[]byte) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{0, 0, 0, 0}) // version + flags
	_ = binary.Write(&payload, binary.BigEndian, uint16(len(entries)))
	for _, entry := range entries {
		payload.Write(entry)
	}
	return buildBox("iinf", payload.Bytes())
}

func buildPITM(itemID uint16) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{0, 0, 0, 0}) // version + flags
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	return buildBox("pitm", payload.Bytes())
}

func buildInfeEXIF(itemID uint16) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{2, 0, 0, 0}) // version 2 + flags
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // protection index
	payload.WriteString("Exif")
	payload.WriteString("PrimaryExif")
	payload.WriteByte(0)
	return buildBox("infe", payload.Bytes())
}

func buildInfeXMP(itemID uint16) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{2, 0, 0, 0}) // version 2 + flags
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // protection index
	payload.WriteString("mime")
	payload.WriteString("PrimaryXMP")
	payload.WriteByte(0)
	payload.WriteString("application/rdf+xml")
	payload.WriteByte(0)
	payload.WriteByte(0) // content encoding
	return buildBox("infe", payload.Bytes())
}

func buildIlocFileOffset(itemID uint16, baseOffset uint32, length uint32) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{1, 0, 0, 0}) // version 1 + flags
	_ = binary.Write(&payload, binary.BigEndian, uint16(0x4440))
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // item count
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // construction method
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // data reference index
	_ = binary.Write(&payload, binary.BigEndian, baseOffset)
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // extent count
	_ = binary.Write(&payload, binary.BigEndian, uint32(0)) // extent offset
	_ = binary.Write(&payload, binary.BigEndian, length)
	return buildBox("iloc", payload.Bytes())
}

func buildIlocIDAT(itemID uint16, length uint32) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{1, 0, 0, 0}) // version 1 + flags
	_ = binary.Write(&payload, binary.BigEndian, uint16(0x4440))
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // item count
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // construction method = idat
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // data reference index
	_ = binary.Write(&payload, binary.BigEndian, uint32(0)) // base offset
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // extent count
	_ = binary.Write(&payload, binary.BigEndian, uint32(0)) // extent offset
	_ = binary.Write(&payload, binary.BigEndian, length)
	return buildBox("iloc", payload.Bytes())
}

func wrapEXIFItemPayload(tiff []byte) []byte {
	var payload bytes.Buffer
	_ = binary.Write(&payload, binary.BigEndian, uint32(6))
	payload.WriteString("Exif\x00\x00")
	payload.Write(tiff)
	return payload.Bytes()
}

func buildMinimalEXIFASCII(tagID uint16, value string) []byte {
	ascii := append([]byte(value), 0)
	buf := make([]byte, 256)
	off := 0
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(buf[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(buf[off:], v)
		off += 4
	}

	buf[0], buf[1] = 'M', 'M'
	off = 2
	put16(0x002A)
	put32(8)

	put16(1) // tag count
	put16(tagID)
	put16(exifTypeASCII)
	put32(uint32(len(ascii)))
	dataOffset := uint32(8 + 2 + 12 + 4)
	put32(dataOffset)
	put32(0) // next IFD

	copy(buf[off:], ascii)
	off += len(ascii)

	return buf[:off]
}

func buildBox(boxType string, payload []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(8+len(payload)))
	buf.WriteString(boxType)
	buf.Write(payload)
	return buf.Bytes()
}
