package videometa

import (
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

	itemPayload := buildMinimalXMPPacket("Item XMP")
	data := buildMP4WithMetaIDATItem(itemPayload, buildInfeXMP(1), buildIlocIDAT(1, uint32(len(itemPayload))))

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: XMP,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.XMP()["CreatorTool"].Value, qt.Equals, "Item XMP")
}
