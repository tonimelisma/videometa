package videometa

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
)

type exifFieldManifestTest struct {
	EXIF    []tagEntryTest `json:"exif"`
	GPS     []tagEntryTest `json:"gps"`
	Interop []tagEntryTest `json:"interop"`
}

type tagEntryTest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Validates: REQ-EXIF-04
func TestEXIFFieldReferenceMatchesManifest(t *testing.T) {
	c := qt.New(t)

	manifest := loadEXIFFieldManifest(c)
	c.Assert(exifFields, qt.DeepEquals, manifestMap(c, manifest.EXIF))
	c.Assert(exifFieldsGPS, qt.DeepEquals, manifestMap(c, manifest.GPS))
	c.Assert(exifInteropFields, qt.DeepEquals, manifestMap(c, manifest.Interop))
	_, hasRemovedTag := exifFields[0x927C]
	c.Assert(hasRemovedTag, qt.IsFalse)
}

func loadEXIFFieldManifest(c *qt.C) exifFieldManifestTest {
	c.Helper()

	data, err := os.ReadFile("gen/exif_fields_reference.json")
	c.Assert(err, qt.IsNil)

	var manifest exifFieldManifestTest
	err = json.Unmarshal(data, &manifest)
	c.Assert(err, qt.IsNil)
	return manifest
}

func manifestMap(c *qt.C, entries []tagEntryTest) map[uint16]string {
	c.Helper()

	result := make(map[uint16]string, len(entries))
	var lastID uint16
	for i, entry := range entries {
		c.Assert(len(entry.ID), qt.Equals, 6)
		c.Assert(entry.ID[:2], qt.Equals, "0x")

		rawValue, err := strconv.ParseUint(entry.ID[2:], 16, 16)
		c.Assert(err, qt.IsNil)
		value := uint16(rawValue)
		if i > 0 {
			c.Assert(value > lastID, qt.IsTrue, qt.Commentf("manifest ids must be strictly ascending"))
		}
		lastID = value
		result[value] = entry.Name
	}
	return result
}
