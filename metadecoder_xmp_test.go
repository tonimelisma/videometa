package videometa

import (
	"io"
	"math"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-XMP-01
func TestDecodeXMPBasic(t *testing.T) {
	c := qt.New(t)

	xmpData := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:tiff="http://ns.adobe.com/tiff/1.0/"
      xmlns:xmp="http://ns.adobe.com/xap/1.0/"
      tiff:Make="Apple"
      tiff:Model="iPhone 15 Pro"
      xmp:CreateDate="2024-06-15T10:30:00">
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

	// Create a minimal video decoder to test XMP parsing.
	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(strings.NewReader("")),
		opts: Options{
			Sources: XMP,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}

	err := d.decodeXMP(strings.NewReader(xmpData))
	c.Assert(err, qt.IsNil)

	c.Assert(tags["Make"].Value, qt.Equals, "Apple")
	c.Assert(tags["Model"].Value, qt.Equals, "iPhone 15 Pro")
	c.Assert(tags["CreateDate"].Value, qt.Equals, "2024-06-15T10:30:00")
	c.Assert(tags["Make"].Source, qt.Equals, XMP)
}

// Validates: REQ-XMP-03
func TestParseXMPGPSCoordinate(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "decimal N", input: "34.0592N", want: 34.0592},
		{name: "decimal S", input: "33.8688S", want: -33.8688},
		{name: "DM format N", input: "26,34.951N", want: 26.582516},
		{name: "DM format W", input: "118,26.760W", want: -118.446},
		{name: "DMS format", input: "34,3,33.12N", want: 34.05920},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			got, err := parseXMPGPSCoordinate(tt.input)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
			c.Assert(math.Abs(got-tt.want) < 0.001, qt.IsTrue,
				qt.Commentf("got %f, want %f", got, tt.want))
		})
	}
}

// Validates: REQ-XMP-01
func TestDecodeXMPWithLists(t *testing.T) {
	c := qt.New(t)

	xmpData := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:subject>
        <rdf:Bag>
          <rdf:li>landscape</rdf:li>
          <rdf:li>sunset</rdf:li>
        </rdf:Bag>
      </dc:subject>
      <dc:creator>
        <rdf:Seq>
          <rdf:li>John Doe</rdf:li>
        </rdf:Seq>
      </dc:creator>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(strings.NewReader("")),
		opts: Options{
			Sources: XMP,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}

	err := d.decodeXMP(strings.NewReader(xmpData))
	c.Assert(err, qt.IsNil)

	// Single creator — should be a string.
	c.Assert(tags["Creator"].Value, qt.Equals, "John Doe")
	// Multiple subjects — should be []string.
	subjects, ok := tags["Subject"].Value.([]string)
	c.Assert(ok, qt.IsTrue)
	c.Assert(subjects, qt.DeepEquals, []string{"landscape", "sunset"})
}

// Validates: REQ-XMP-05
func TestDecodeXMPExtendedSkip(t *testing.T) {
	c := qt.New(t)

	// XMP with xmpNote:HasExtendedXMP attribute — main packet should parse fine,
	// the attribute should be emitted as a tag, and no error should occur despite
	// the extended XMP data not being present.
	xmpData := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:tiff="http://ns.adobe.com/tiff/1.0/"
      xmlns:xmpNote="http://ns.adobe.com/xmp/note/"
      tiff:Make="MainPacket"
      xmpNote:HasExtendedXMP="abc123def456">
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

	tags := make(map[string]TagInfo)
	bd := &baseDecoder{
		streamReader: newStreamReader(strings.NewReader("")),
		opts: Options{
			Sources: XMP,
			HandleTag: func(ti TagInfo) error {
				tags[ti.Tag] = ti
				return nil
			},
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}

	err := d.decodeXMP(strings.NewReader(xmpData))
	c.Assert(err, qt.IsNil)

	// Main packet tags are present.
	c.Assert(tags["Make"].Value, qt.Equals, "MainPacket")
	// HasExtendedXMP is emitted as a normal attribute tag.
	c.Assert(tags["HasExtendedXMP"].Value, qt.Equals, "abc123def456")
}

// Validates: REQ-XMP-06
func TestDecodeXMPHandleXMPEscapeHatch(t *testing.T) {
	c := qt.New(t)

	xmpData := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/></x:xmpmeta>`

	var customCalled bool
	bd := &baseDecoder{
		streamReader: newStreamReader(strings.NewReader("")),
		opts: Options{
			Sources: XMP,
			HandleXMP: func(r io.Reader) error {
				customCalled = true
				return nil
			},
			HandleTag: func(ti TagInfo) error { return nil },
		},
		result: &DecodeResult{},
	}
	d := &videoDecoderMP4{baseDecoder: bd}

	err := d.decodeXMP(strings.NewReader(xmpData))
	c.Assert(err, qt.IsNil)
	c.Assert(customCalled, qt.IsTrue)
}
