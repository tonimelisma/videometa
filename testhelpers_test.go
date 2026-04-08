package videometa

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
)

// readerSeekerFromBytes creates an io.ReadSeeker from a byte slice.
func readerSeekerFromBytes(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
}

func decodeAllForTest(opts Options) (Tags, VideoConfig, error) {
	metadata, err := DecodeAll(opts)
	return metadata.Tags, metadata.VideoConfig, err
}

func flattenSourceTags(sourceTags SourceTags) map[string]TagInfo {
	return flattenSourceTagsWhere(sourceTags, func(TagInfo) bool { return true })
}

func flattenSourceTagsWhere(sourceTags SourceTags, keep func(TagInfo) bool) map[string]TagInfo {
	flat := make(map[string]TagInfo)
	for _, tag := range sourceTags.All() {
		if keep(tag) {
			flat[tag.Tag] = tag
		}
	}
	return flat
}

func flattenAllTags(tags Tags) map[string]TagInfo {
	flat := make(map[string]TagInfo)
	for _, tag := range tags.All() {
		flat[tag.Tag] = tag
	}
	return flat
}

func firstTagInNamespace(sourceTags SourceTags, namespace string, tag string) (TagInfo, bool) {
	matches := sourceTags.FindInNamespace(namespace, tag)
	if len(matches) == 0 {
		return TagInfo{}, false
	}
	return matches[0], true
}

func buildMP4WithSonyNRTMIDAT(payload []byte) []byte {
	ftyp := buildFTYPBox()
	meta := buildMetaBox(buildMetaHdlr("nrtm", "Sony NRTM"), buildBox("idat", payload))
	return append(ftyp, meta...)
}

func buildMP4WithLargeMdat(payloadSize int) []byte {
	ftyp := buildFTYPBox()
	mdat := buildBox("mdat", bytes.Repeat([]byte("M"), payloadSize))
	moov := buildBox("moov", buildMinimalMvhdPayload(1000, 5000))
	return append(append(ftyp, mdat...), moov...)
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

func buildMetaHdlr(handlerType string, name string) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{0, 0, 0, 0})
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	payload.WriteString(handlerType)
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	if name != "" {
		payload.WriteString(name)
	}
	payload.WriteByte(0)
	return buildBox("hdlr", payload.Bytes())
}

func buildBox(boxType string, payload []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(8+len(payload)))
	buf.WriteString(boxType)
	buf.Write(payload)
	return buf.Bytes()
}

func buildMinimalMvhdPayload(timescale uint32, duration uint32) []byte {
	var payload bytes.Buffer
	payload.Write([]byte{0, 0, 0, 0})
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	_ = binary.Write(&payload, binary.BigEndian, timescale)
	_ = binary.Write(&payload, binary.BigEndian, duration)
	_ = binary.Write(&payload, binary.BigEndian, uint32(0x00010000))
	_ = binary.Write(&payload, binary.BigEndian, uint16(0x0100))
	payload.Write(make([]byte, 10))

	matrix := make([]byte, 36)
	matrix[3] = 1
	matrix[4+12+3] = 1
	matrix[8+24] = 0x40
	payload.Write(matrix)

	payload.Write(make([]byte, 24))
	_ = binary.Write(&payload, binary.BigEndian, uint32(1))
	return buildBox("mvhd", payload.Bytes())
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
