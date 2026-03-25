package videometa

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
)

type testEXIFEntry struct {
	tagID uint16
	typ   uint16
	data  []byte
}

type testIlocExtent struct {
	offset uint32
	length uint32
}

// readerSeekerFromBytes creates an io.ReadSeeker from a byte slice.
func readerSeekerFromBytes(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
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

func buildMP4WithXMPUUID(xmpPayload []byte) []byte {
	var buf bytes.Buffer
	buf.Write(buildFTYPBox())

	_ = binary.Write(&buf, binary.BigEndian, uint32(8+16+len(xmpPayload)))
	buf.WriteString("uuid")
	buf.Write(xmpUUID[:])
	buf.Write(xmpPayload)

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
	return buildIloc(itemID, 0, baseOffset, testIlocExtent{
		offset: 0,
		length: length,
	})
}

func buildIlocIDAT(itemID uint16, length uint32) []byte {
	return buildIloc(itemID, 1, 0, testIlocExtent{
		offset: 0,
		length: length,
	})
}

func buildIloc(itemID uint16, constructionMethod uint16, baseOffset uint32, extents ...testIlocExtent) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{1, 0, 0, 0}) // version 1 + flags
	_ = binary.Write(&payload, binary.BigEndian, uint16(0x4440))
	_ = binary.Write(&payload, binary.BigEndian, uint16(1)) // item count
	_ = binary.Write(&payload, binary.BigEndian, itemID)
	_ = binary.Write(&payload, binary.BigEndian, constructionMethod)
	_ = binary.Write(&payload, binary.BigEndian, uint16(0)) // data reference index
	_ = binary.Write(&payload, binary.BigEndian, baseOffset)
	_ = binary.Write(&payload, binary.BigEndian, uint16(len(extents)))
	for _, extent := range extents {
		_ = binary.Write(&payload, binary.BigEndian, extent.offset)
		_ = binary.Write(&payload, binary.BigEndian, extent.length)
	}
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
	return buildTIFFWithEntries([]testEXIFEntry{
		{
			tagID: tagID,
			typ:   exifTypeASCII,
			data:  append([]byte(value), 0),
		},
	})
}

// buildMP4WithInvalidEXIF builds a minimal valid MP4 with ftyp + uuid (EXIF)
// box containing garbage data. This triggers the EXIF decoder's Warnf callback
// for invalid byte order marker.
func buildMP4WithInvalidEXIF() []byte {
	var buf bytes.Buffer

	_ = binary.Write(&buf, binary.BigEndian, uint32(20))
	buf.WriteString("ftyp")
	buf.WriteString("isom")
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	buf.WriteString("isom")

	exifBody := make([]byte, 12)
	binary.BigEndian.PutUint32(exifBody[0:4], 0)
	copy(exifBody[4:], []byte("BADEXIF!"))

	boxSize := uint32(8 + 16 + len(exifBody))
	_ = binary.Write(&buf, binary.BigEndian, boxSize)
	buf.WriteString("uuid")
	buf.Write(exifUUID[:])
	buf.Write(exifBody)

	return buf.Bytes()
}

// buildEXIFWithMakerNotes creates an EXIF/TIFF structure with a MakerNotes tag (0x927C).
func buildEXIFWithMakerNotes(makerNotesData []byte) []byte {
	return buildTIFFWithEntries([]testEXIFEntry{
		{
			tagID: 0x927C,
			typ:   exifTypeUndef,
			data:  makerNotesData,
		},
	})
}

// buildEXIFWithIPTC creates an EXIF/TIFF structure with ApplicationNotes tag (0x83BB)
// containing IPTC data.
func buildEXIFWithIPTC(iptcData []byte) []byte {
	return buildTIFFWithEntries([]testEXIFEntry{
		{
			tagID: 0x83BB,
			typ:   exifTypeUndef,
			data:  iptcData,
		},
	})
}

func buildEXIFWithMakeModelAndMakerNotes(makeValue, modelValue string, makerNotesData []byte) []byte {
	entries := []testEXIFEntry{
		{
			tagID: 0x010F,
			typ:   exifTypeASCII,
			data:  append([]byte(makeValue), 0),
		},
	}
	if modelValue != "" {
		entries = append(entries, testEXIFEntry{
			tagID: 0x0110,
			typ:   exifTypeASCII,
			data:  append([]byte(modelValue), 0),
		})
	}
	entries = append(entries, testEXIFEntry{
		tagID: 0x927C,
		typ:   exifTypeUndef,
		data:  makerNotesData,
	})
	return buildTIFFWithEntries(entries)
}

func buildTIFFWithEntries(entries []testEXIFEntry) []byte {
	buf := make([]byte, 4096)
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

	put16(uint16(len(entries)))
	dataOffset := 8 + 2 + len(entries)*12 + 4
	currentDataOffset := dataOffset
	for _, entry := range entries {
		put16(entry.tagID)
		put16(entry.typ)
		put32(uint32(len(entry.data)))
		put32(uint32(currentDataOffset))
		currentDataOffset += len(entry.data)
	}
	put32(0)

	for _, entry := range entries {
		copy(buf[off:], entry.data)
		off += len(entry.data)
	}

	return buf[:off]
}

func buildAppleMakerNotePayload(contentIdentifier string) []byte {
	header := append([]byte("Apple iOS\x00"), 0x00, 0x00, 0x00, 0x00)
	payload := make([]byte, 256)
	copy(payload, header)

	off := len(header)
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	value := append([]byte(contentIdentifier), 0)
	put16(1)
	put16(0x0011)
	put16(exifTypeASCII)
	put32(uint32(len(value)))
	put32(uint32(len(header) + 2 + 12 + 4))
	put32(0)
	copy(payload[off:], value)
	off += len(value)

	return payload[:off]
}

func buildCanonMakerNotePayload() []byte {
	cameraSettings := make([]uint16, 34)
	cameraSettings[6] = 5   // FocusMode
	cameraSettings[15] = 17 // CameraISO => 100
	cameraSettings[24] = 10 // FocalUnits
	cameraSettings[33] = 1  // ImageStabilization

	shotInfo := make([]int16, 22)
	shotInfo[20] = 0x0080 // FNumber => 4.0
	shotInfo[21] = 0x0060 // ExposureTime => 1/8

	payload := make([]byte, 256)
	off := 0
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	put16(2)
	put16(0x0001)
	put16(exifTypeShort)
	put32(uint32(len(cameraSettings)))
	put32(30)

	put16(0x0004)
	put16(exifTypeSShort)
	put32(uint32(len(shotInfo)))
	put32(30 + uint32(len(cameraSettings))*2)

	put32(0)

	for _, v := range cameraSettings {
		put16(v)
	}
	for _, v := range shotInfo {
		put16(uint16(v))
	}

	return payload[:off]
}

func buildSonyMakerNotePayload() []byte {
	header := append([]byte("SONY DSC \x00"), 0x00, 0x00)
	payload := make([]byte, 256)
	copy(payload, header)

	off := len(header)
	put16 := func(v uint16) {
		binary.BigEndian.PutUint16(payload[off:], v)
		off += 2
	}
	put32 := func(v uint32) {
		binary.BigEndian.PutUint32(payload[off:], v)
		off += 4
	}

	put16(3)
	put16(0x0102)
	put16(exifTypeLong)
	put32(1)
	put32(5)

	put16(0x0104)
	put16(exifTypeSRational)
	put32(1)
	put32(54)

	put16(0x0115)
	put16(exifTypeLong)
	put32(1)
	put32(0x20)

	put32(0)
	binary.BigEndian.PutUint32(payload[off:], 1)
	off += 4
	binary.BigEndian.PutUint32(payload[off:], 2)
	off += 4

	return payload[:off]
}

func buildMinimalXMPPacket(creatorTool string) []byte {
	return []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:CreatorTool="` + creatorTool + `"/></rdf:RDF></x:xmpmeta>`)
}

func buildBox(boxType string, payload []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(8+len(payload)))
	buf.WriteString(boxType)
	buf.Write(payload)
	return buf.Bytes()
}

// slowReader wraps an io.ReadSeeker and adds delay to each Read call.
type slowReader struct {
	rs    io.ReadSeeker
	delay time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	time.Sleep(s.delay)
	return s.rs.Read(p)
}

func (s *slowReader) Seek(offset int64, whence int) (int64, error) {
	return s.rs.Seek(offset, whence)
}
