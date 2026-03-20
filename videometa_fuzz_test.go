package videometa

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// Validates: REQ-NF-05
func FuzzDecodeMP4(f *testing.F) {
	// Seed corpus from test files.
	seeds := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/truncated.mp4",
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		"testdata/sony_a6700.mp4",
		"testdata/apple.mov",
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, err := Decode(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
			HandleTag: func(ti TagInfo) error {
				return nil
			},
		})
		// All errors from malformed input must be InvalidFormatError.
		if err != nil && !IsInvalidFormat(err) {
			t.Errorf("expected InvalidFormatError, got: %T: %v", err, err)
		}
	})
}

// Validates: REQ-NF-05
func FuzzDecodeAllMP4(f *testing.F) {
	seeds := []string{
		"testdata/minimal.mp4",
		"testdata/nonfaststart.mp4",
		"testdata/truncated.mp4",
		"testdata/with_audio.mp4",
		"testdata/with_gps.mp4",
		"testdata/exiftool_quicktime.mov",
		"testdata/sony_a6700.mp4",
		"testdata/apple.mov",
	}
	for _, path := range seeds {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, _, err := DecodeAll(Options{
			R:       r,
			Sources: EXIF | XMP | IPTC | QUICKTIME | CONFIG,
		})
		// All errors from malformed input must be InvalidFormatError.
		if err != nil && !IsInvalidFormat(err) {
			t.Errorf("expected InvalidFormatError, got: %T: %v", err, err)
		}
	})
}

// Validates: REQ-NF-05
func FuzzDecodeEXIF(f *testing.F) {
	// Seed: valid TIFF with a MakerNotes tag.
	f.Add(buildEXIFWithMakerNotes([]byte("TestMaker\x00\x00\x00\x00\x00\x00\x00")))
	// Seed: minimal big-endian TIFF header.
	f.Add([]byte("MM\x00\x2A\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00"))

	f.Fuzz(func(t *testing.T, data []byte) {
		bd := &baseDecoder{
			streamReader: newStreamReader(bytes.NewReader(nil)),
			opts: Options{
				Sources:   EXIF | MAKERNOTES,
				HandleTag: func(ti TagInfo) error { return nil },
			},
			result: &DecodeResult{},
		}
		d := &videoDecoderMP4{baseDecoder: bd}
		// Must not panic on any input — decodeEXIF has its own recovery.
		d.decodeEXIF(bytes.NewReader(data))
	})
}

// Validates: REQ-NF-05
func FuzzDecodeXMP(f *testing.F) {
	// Seed: minimal valid XMP.
	f.Add([]byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" dc:creator="test"/></rdf:RDF></x:xmpmeta>`))
	// Seed: garbage.
	f.Add([]byte("not xml at all {{{"))

	f.Fuzz(func(t *testing.T, data []byte) {
		bd := &baseDecoder{
			streamReader: newStreamReader(bytes.NewReader(nil)),
			opts: Options{
				Sources:   XMP,
				HandleTag: func(ti TagInfo) error { return nil },
			},
			result: &DecodeResult{},
		}
		d := &videoDecoderMP4{baseDecoder: bd}
		// Must not panic — decodeXMP returns error gracefully.
		_ = d.decodeXMP(bytes.NewReader(data))
	})
}

// Validates: REQ-NF-05
func FuzzDecodeIPTC(f *testing.F) {
	// Seed: minimal valid IPTC record (marker + record 2 + dataset 5 ObjectName).
	var iptcSeed bytes.Buffer
	iptcSeed.WriteByte(0x1C) // marker
	iptcSeed.WriteByte(2)    // record 2
	iptcSeed.WriteByte(5)    // dataset 5 = ObjectName
	_ = binary.Write(&iptcSeed, binary.BigEndian, uint16(4))
	iptcSeed.WriteString("Test")
	f.Add(iptcSeed.Bytes())
	// Seed: garbage.
	f.Add([]byte("\x00\xFF\xFF\xFF"))

	f.Fuzz(func(t *testing.T, data []byte) {
		bd := &baseDecoder{
			streamReader: newStreamReader(bytes.NewReader(nil)),
			opts: Options{
				Sources:   IPTC,
				HandleTag: func(ti TagInfo) error { return nil },
			},
			result: &DecodeResult{},
		}
		d := &videoDecoderMP4{baseDecoder: bd}
		// Must not panic — decodeIPTC has its own recovery.
		d.decodeIPTC(bytes.NewReader(data))
	})
}
