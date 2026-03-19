package videometa

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

// Validates: REQ-NF-04
func TestNrtmParseNumeric(t *testing.T) {
	c := qt.New(t)

	c.Assert(nrtmParseNumeric("42"), qt.Equals, 42)
	c.Assert(nrtmParseNumeric("3.14"), qt.Equals, 3.14)
	c.Assert(nrtmParseNumeric("hello"), qt.Equals, "hello")
	c.Assert(nrtmParseNumeric("0"), qt.Equals, 0)
}

// Validates: REQ-NF-04
func TestNrtmParseBool(t *testing.T) {
	c := qt.New(t)

	c.Assert(nrtmParseBool("true"), qt.Equals, true)
	c.Assert(nrtmParseBool("false"), qt.Equals, false)
	c.Assert(nrtmParseBool("True"), qt.Equals, true)
	c.Assert(nrtmParseBool("maybe"), qt.Equals, "maybe")
}

// Validates: REQ-NF-04
func TestScanForXMLInMeta(t *testing.T) {
	c := qt.New(t)

	// XML found after binary prefix.
	data := []byte("BINARY_JUNK\x00\x00<?xml version=\"1.0\"?><root/>")
	result := scanForXMLInMeta(data)
	c.Assert(result, qt.IsNotNil)
	c.Assert(string(result[:5]), qt.Equals, "<?xml")

	// No XML.
	c.Assert(scanForXMLInMeta([]byte("no xml here")), qt.IsNil)

	// Empty.
	c.Assert(scanForXMLInMeta([]byte{}), qt.IsNil)
}

// Validates: REQ-NF-04
func TestDecodeSonyNRTM(t *testing.T) {
	c := qt.New(t)

	xml := `<?xml version="1.0" encoding="UTF-8"?>
<NonRealTimeMeta xmlns="urn:schemas-professionalDisc:nonRealTimeMeta:ver.2.20" lastUpdate="2026-01-01T12:00:00Z">
	<TargetMaterial umidRef="ABCD1234"/>
	<Duration value="100"/>
	<LtcChangeTable tcFps="24" halfStep="false">
		<LtcChange frameCount="0" value="12345" status="increment"/>
	</LtcChangeTable>
	<CreationDate value="2026-01-01T12:00:00Z"/>
	<VideoFormat>
		<VideoRecPort port="HDMI"/>
		<VideoFrame videoCodec="H264" captureFps="24p" formatFps="24p"/>
		<VideoLayout pixel="1920" numOfVerticalLine="1080" aspectRatio="16:9"/>
	</VideoFormat>
	<AudioFormat numOfChannel="2">
		<AudioRecPort port="XLR" audioCodec="LPCM24" trackDst="CH1"/>
	</AudioFormat>
	<Device manufacturer="Sony" modelName="TestCam" serialNo="12345"/>
	<RecordingMode type="normal" cacheRec="true"/>
	<AcquisitionRecord>
		<Group name="TestGroup">
			<Item name="Key1" value="Val1"/>
		</Group>
		<ChangeTable name="TestTable">
			<Event frameCount="0" status="start"/>
		</ChangeTable>
	</AcquisitionRecord>
</NonRealTimeMeta>`

	var tags []TagInfo
	bd := &baseDecoder{
		streamReader: newStreamReader(nil),
		opts: Options{
			Sources: XML,
			HandleTag: func(ti TagInfo) error {
				tags = append(tags, ti)
				return nil
			},
		},
		result: &DecodeResult{},
	}
	dec := &videoDecoderMP4{baseDecoder: bd}
	dec.decodeSonyNRTM(strings.NewReader(xml))

	tagMap := make(map[string]any)
	for _, ti := range tags {
		c.Assert(ti.Source, qt.Equals, XML)
		c.Assert(ti.Namespace, qt.Equals, "XML")
		tagMap[ti.Tag] = ti.Value
	}

	c.Assert(tagMap["LastUpdate"], qt.Equals, "2026-01-01T12:00:00Z")
	c.Assert(tagMap["TargetMaterialUmidRef"], qt.Equals, "ABCD1234")
	c.Assert(tagMap["DurationValue"], qt.Equals, 100)
	c.Assert(tagMap["LtcChangeTableTcFps"], qt.Equals, 24)
	c.Assert(tagMap["LtcChangeTableHalfStep"], qt.Equals, false)
	c.Assert(tagMap["LtcChangeTableLtcChangeFrameCount"], qt.Equals, 0)
	c.Assert(tagMap["LtcChangeTableLtcChangeValue"], qt.Equals, 12345)
	c.Assert(tagMap["LtcChangeTableLtcChangeStatus"], qt.Equals, "increment")
	c.Assert(tagMap["CreationDateValue"], qt.Equals, "2026-01-01T12:00:00Z")
	c.Assert(tagMap["VideoFormatVideoRecPortPort"], qt.Equals, "HDMI")
	c.Assert(tagMap["VideoFormatVideoFrameVideoCodec"], qt.Equals, "H264")
	c.Assert(tagMap["VideoFormatVideoLayoutPixel"], qt.Equals, 1920)
	c.Assert(tagMap["VideoFormatVideoLayoutNumOfVerticalLine"], qt.Equals, 1080)
	c.Assert(tagMap["VideoFormatVideoLayoutAspectRatio"], qt.Equals, "16:9")
	c.Assert(tagMap["AudioFormatNumOfChannel"], qt.Equals, 2)
	c.Assert(tagMap["AudioFormatAudioRecPortPort"], qt.Equals, "XLR")
	c.Assert(tagMap["DeviceManufacturer"], qt.Equals, "Sony")
	c.Assert(tagMap["DeviceModelName"], qt.Equals, "TestCam")
	c.Assert(tagMap["DeviceSerialNo"], qt.Equals, 12345)
	c.Assert(tagMap["RecordingModeType"], qt.Equals, "normal")
	c.Assert(tagMap["RecordingModeCacheRec"], qt.Equals, true)
	c.Assert(tagMap["AcquisitionRecordGroupName"], qt.Equals, "TestGroup")
	c.Assert(tagMap["AcquisitionRecordGroupItemName"], qt.Equals, "Key1")
	c.Assert(tagMap["AcquisitionRecordGroupItemValue"], qt.Equals, "Val1")
	c.Assert(tagMap["AcquisitionRecordChangeTableName"], qt.Equals, "TestTable")
	c.Assert(tagMap["AcquisitionRecordChangeTableEventFrameCount"], qt.Equals, 0)
	c.Assert(tagMap["AcquisitionRecordChangeTableEventStatus"], qt.Equals, "start")
}
