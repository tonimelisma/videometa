package videometa

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
)

type countingReadSeeker struct {
	*bytes.Reader
	bytesRead int64
}

func newCountingReadSeeker(data []byte) *countingReadSeeker {
	return &countingReadSeeker{
		Reader: bytes.NewReader(data),
	}
}

func (r *countingReadSeeker) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytesRead += int64(n)
	return n, err
}

func minimalSonyNRTMXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<NonRealTimeMeta xmlns="urn:schemas-professionalDisc:nonRealTimeMeta:ver.2.20" lastUpdate="2026-01-01T12:00:00Z">
	<Device manufacturer="Sony" modelName="RouteCam" serialNo="12345"/>
</NonRealTimeMeta>`
}

// Validates: REQ-NF-04
func TestDecodeSonyNRTMFromMetaIDAT(t *testing.T) {
	c := qt.New(t)

	payload := append([]byte("BINARY_PREFIX\x00\x00"), []byte(minimalSonyNRTMXML())...)
	data := buildMP4WithSonyNRTMIDAT(payload)

	tags, _, err := decodeAllForTest(Options{
		R:       readerSeekerFromBytes(data),
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)
	vendorTags := flattenSourceTags(tags.Vendor())
	c.Assert(vendorTags["DeviceManufacturer"].Value, qt.Equals, "Sony")
	c.Assert(vendorTags["DeviceModelName"].Value, qt.Equals, "RouteCam")
	c.Assert(vendorTags["DeviceSerialNo"].Value, qt.Equals, 12345)
}

// Validates: REQ-NF-01
func TestDecodeSonyNRTMLargeMetaIDATDoesNotBufferForXMLOnly(t *testing.T) {
	c := qt.New(t)

	payload := bytes.Repeat([]byte("N"), 2<<20)
	data := buildMP4WithSonyNRTMIDAT(payload)
	reader := newCountingReadSeeker(data)

	tags, _, err := decodeAllForTest(Options{
		R:       reader,
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(tags.Vendor().All()), qt.Equals, 0)
	c.Assert(reader.bytesRead < 128*1024, qt.IsTrue,
		qt.Commentf("expected large idat to be skipped via seek without reading payload, read %d bytes", reader.bytesRead))
}

// Validates: REQ-NF-01
func TestDecodeSonyNRTMLargeMetaIDATReaderOnlyStillStreams(t *testing.T) {
	c := qt.New(t)

	payload := bytes.Repeat([]byte("N"), 2<<20)
	data := buildMP4WithSonyNRTMIDAT(payload)

	tags, _, err := decodeAllForTest(Options{
		R:       readerOnly{bytes.NewReader(data)},
		Sources: VENDOR,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(tags.Vendor().All()), qt.Equals, 0)
}
