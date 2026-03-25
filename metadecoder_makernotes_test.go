package videometa

import (
	"math"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-EXIF-07
func TestDecodeAppleMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("Apple", "", buildAppleMakerNotePayload("LIVE-ID-123"))
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(tags.MakerNotes()["ContentIdentifier"].Value, qt.Equals, "LIVE-ID-123")
}

// Validates: REQ-EXIF-08
func TestDecodeCanonMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("Canon", "EOS R5", buildCanonMakerNotePayload())
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(tags.MakerNotes()["FocusMode"].Value, qt.Equals, 5)
	c.Assert(tags.MakerNotes()["CameraISO"].Value, qt.Equals, 100)
	c.Assert(tags.MakerNotes()["ImageStabilization"].Value, qt.Equals, 1)

	fNumber, ok := tags.MakerNotes()["FNumber"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(fNumber-4.0) < 1e-9, qt.IsTrue)

	exposureTime, ok := tags.MakerNotes()["ExposureTime"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(exposureTime-0.125) < 1e-9, qt.IsTrue)
}

// Validates: REQ-EXIF-09
func TestDecodeSonyMakerNotes(t *testing.T) {
	c := qt.New(t)

	exifPayload := buildEXIFWithMakeModelAndMakerNotes("SONY", "ILCE-7M4", buildSonyMakerNotePayload())
	data := buildMP4WithEXIFUUID(exifPayload)

	tags, _, err := DecodeAll(Options{
		R:       readerSeekerFromBytes(data),
		Sources: EXIF | MAKERNOTES,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(tags.MakerNotes()["Quality"].Value, qt.Equals, uint32(5))
	c.Assert(tags.MakerNotes()["WhiteBalance"].Value, qt.Equals, uint32(0x20))

	flashExposureComp, ok := tags.MakerNotes()["FlashExposureComp"].Value.(float64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(math.Abs(flashExposureComp-0.5) < 1e-9, qt.IsTrue)
}
