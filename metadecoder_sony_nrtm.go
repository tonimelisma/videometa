package videometa

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// decodeSonyNRTM parses Sony NonRealTimeMeta XML and emits tags matching
// exiftool's XML group output. The tag naming convention flattens the XML
// hierarchy: parent element names are concatenated with attribute/child names
// in CamelCase. Only the first occurrence of repeating elements is emitted.
func (d *videoDecoderMP4) decodeSonyNRTM(r io.Reader) {
	dec := xml.NewDecoder(r)

	var root nrtmRoot
	if err := dec.Decode(&root); err != nil {
		if d.opts.Warnf != nil {
			d.opts.Warnf("decode sony nrtm: %v", err)
		}
		return
	}

	emit := func(name string, value any) {
		d.emitTag(TagInfo{
			Source:    XML,
			Tag:       name,
			Namespace: "XML",
			Value:     value,
		})
	}

	if root.LastUpdate != "" {
		emit("LastUpdate", root.LastUpdate)
	}
	if root.TargetMaterial.UmidRef != "" {
		emit("TargetMaterialUmidRef", root.TargetMaterial.UmidRef)
	}
	if root.Duration.Value != "" {
		emit("DurationValue", nrtmParseNumeric(root.Duration.Value))
	}

	// LtcChangeTable — emit table attributes + first LtcChange entry.
	if root.LtcChangeTable.TcFps != "" {
		emit("LtcChangeTableTcFps", nrtmParseNumeric(root.LtcChangeTable.TcFps))
	}
	if root.LtcChangeTable.HalfStep != "" {
		emit("LtcChangeTableHalfStep", nrtmParseBool(root.LtcChangeTable.HalfStep))
	}
	if len(root.LtcChangeTable.LtcChanges) > 0 {
		lc := root.LtcChangeTable.LtcChanges[0]
		emit("LtcChangeTableLtcChangeFrameCount", nrtmParseNumeric(lc.FrameCount))
		emit("LtcChangeTableLtcChangeValue", nrtmParseNumeric(lc.Value))
		if lc.Status != "" {
			emit("LtcChangeTableLtcChangeStatus", lc.Status)
		}
	}

	if root.CreationDate.Value != "" {
		emit("CreationDateValue", root.CreationDate.Value)
	}

	// KlvPacketTable — first entry.
	if len(root.KlvPacketTable.KlvPackets) > 0 {
		kp := root.KlvPacketTable.KlvPackets[0]
		if kp.Key != "" {
			emit("KlvPacketTableKlvPacketKey", kp.Key)
		}
		emit("KlvPacketTableKlvPacketFrameCount", nrtmParseNumeric(kp.FrameCount))
		if kp.LengthValue != "" {
			emit("KlvPacketTableKlvPacketLengthValue", kp.LengthValue)
		}
		if kp.Status != "" {
			emit("KlvPacketTableKlvPacketStatus", kp.Status)
		}
	}

	// VideoFormat.
	if root.VideoFormat.VideoRecPort.Port != "" {
		emit("VideoFormatVideoRecPortPort", root.VideoFormat.VideoRecPort.Port)
	}
	vf := root.VideoFormat.VideoFrame
	if vf.VideoCodec != "" {
		emit("VideoFormatVideoFrameVideoCodec", vf.VideoCodec)
	}
	if vf.CaptureFps != "" {
		emit("VideoFormatVideoFrameCaptureFps", vf.CaptureFps)
	}
	if vf.FormatFps != "" {
		emit("VideoFormatVideoFrameFormatFps", vf.FormatFps)
	}
	vl := root.VideoFormat.VideoLayout
	if vl.Pixel != "" {
		emit("VideoFormatVideoLayoutPixel", nrtmParseNumeric(vl.Pixel))
	}
	if vl.NumOfVerticalLine != "" {
		emit("VideoFormatVideoLayoutNumOfVerticalLine", nrtmParseNumeric(vl.NumOfVerticalLine))
	}
	if vl.AspectRatio != "" {
		emit("VideoFormatVideoLayoutAspectRatio", vl.AspectRatio)
	}

	// AudioFormat.
	if root.AudioFormat.NumOfChannel != "" {
		emit("AudioFormatNumOfChannel", nrtmParseNumeric(root.AudioFormat.NumOfChannel))
	}
	if len(root.AudioFormat.AudioRecPorts) > 0 {
		arp := root.AudioFormat.AudioRecPorts[0]
		if arp.Port != "" {
			emit("AudioFormatAudioRecPortPort", arp.Port)
		}
		if arp.AudioCodec != "" {
			emit("AudioFormatAudioRecPortAudioCodec", arp.AudioCodec)
		}
		if arp.TrackDst != "" {
			emit("AudioFormatAudioRecPortTrackDst", arp.TrackDst)
		}
	}

	// Device.
	if root.Device.Manufacturer != "" {
		emit("DeviceManufacturer", root.Device.Manufacturer)
	}
	if root.Device.ModelName != "" {
		emit("DeviceModelName", root.Device.ModelName)
	}
	if root.Device.SerialNo != "" {
		emit("DeviceSerialNo", nrtmParseNumeric(root.Device.SerialNo))
	}

	// RecordingMode.
	if root.RecordingMode.Type != "" {
		emit("RecordingModeType", root.RecordingMode.Type)
	}
	if root.RecordingMode.CacheRec != "" {
		emit("RecordingModeCacheRec", nrtmParseBool(root.RecordingMode.CacheRec))
	}

	// AcquisitionRecord — first Group + first ChangeTable.
	if len(root.AcquisitionRecord.Groups) > 0 {
		g := root.AcquisitionRecord.Groups[0]
		if g.Name != "" {
			emit("AcquisitionRecordGroupName", g.Name)
		}
		if len(g.Items) > 0 {
			item := g.Items[0]
			if item.Name != "" {
				emit("AcquisitionRecordGroupItemName", item.Name)
			}
			if item.Value != "" {
				emit("AcquisitionRecordGroupItemValue", item.Value)
			}
		}
	}
	if len(root.AcquisitionRecord.ChangeTables) > 0 {
		ct := root.AcquisitionRecord.ChangeTables[0]
		if ct.Name != "" {
			emit("AcquisitionRecordChangeTableName", ct.Name)
		}
		if len(ct.Events) > 0 {
			ev := ct.Events[0]
			emit("AcquisitionRecordChangeTableEventFrameCount", nrtmParseNumeric(ev.FrameCount))
			if ev.Status != "" {
				emit("AcquisitionRecordChangeTableEventStatus", ev.Status)
			}
		}
	}
}

// nrtmParseNumeric attempts to parse a string as int or float64, returning
// the typed value. Falls back to the raw string.
func nrtmParseNumeric(s string) any {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// nrtmParseBool parses "true"/"false" strings to bool.
func nrtmParseBool(s string) any {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	default:
		return s
	}
}

// scanForXMLInMeta finds the XML start within a meta/idat box by looking
// for the <?xml marker. Returns an io.Reader starting at that position, or
// nil if not found. The reader is limited to maxLen bytes from the scan start.
func scanForXMLInMeta(data []byte) []byte {
	marker := []byte("<?xml")
	idx := -1
	for i := 0; i+len(marker) <= len(data); i++ {
		if data[i] == '<' && string(data[i:i+len(marker)]) == string(marker) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	return data[idx:]
}

// nrtmRoot mirrors the top-level NonRealTimeMeta XML structure.
type nrtmRoot struct {
	XMLName           xml.Name        `xml:"NonRealTimeMeta"`
	LastUpdate        string          `xml:"lastUpdate,attr"`
	TargetMaterial    nrtmTarget      `xml:"TargetMaterial"`
	Duration          nrtmDuration    `xml:"Duration"`
	LtcChangeTable    nrtmLtcTable    `xml:"LtcChangeTable"`
	CreationDate      nrtmDuration    `xml:"CreationDate"`
	KlvPacketTable    nrtmKlvTable    `xml:"KlvPacketTable"`
	VideoFormat       nrtmVideoFormat `xml:"VideoFormat"`
	AudioFormat       nrtmAudioFormat `xml:"AudioFormat"`
	Device            nrtmDevice      `xml:"Device"`
	RecordingMode     nrtmRecMode     `xml:"RecordingMode"`
	AcquisitionRecord nrtmAcqRecord   `xml:"AcquisitionRecord"`
}

type nrtmTarget struct {
	UmidRef string `xml:"umidRef,attr"`
}

type nrtmDuration struct {
	Value string `xml:"value,attr"`
}

type nrtmLtcTable struct {
	TcFps      string          `xml:"tcFps,attr"`
	HalfStep   string          `xml:"halfStep,attr"`
	LtcChanges []nrtmLtcChange `xml:"LtcChange"`
}

type nrtmLtcChange struct {
	FrameCount string `xml:"frameCount,attr"`
	Value      string `xml:"value,attr"`
	Status     string `xml:"status,attr"`
}

type nrtmKlvTable struct {
	KlvPackets []nrtmKlvPacket `xml:"KlvPacket"`
}

type nrtmKlvPacket struct {
	Key         string `xml:"key,attr"`
	FrameCount  string `xml:"frameCount,attr"`
	LengthValue string `xml:"lengthValue,attr"`
	Status      string `xml:"status,attr"`
}

type nrtmVideoFormat struct {
	VideoRecPort nrtmPort        `xml:"VideoRecPort"`
	VideoFrame   nrtmVideoFrame  `xml:"VideoFrame"`
	VideoLayout  nrtmVideoLayout `xml:"VideoLayout"`
}

type nrtmPort struct {
	Port string `xml:"port,attr"`
}

type nrtmVideoFrame struct {
	VideoCodec string `xml:"videoCodec,attr"`
	CaptureFps string `xml:"captureFps,attr"`
	FormatFps  string `xml:"formatFps,attr"`
}

type nrtmVideoLayout struct {
	Pixel             string `xml:"pixel,attr"`
	NumOfVerticalLine string `xml:"numOfVerticalLine,attr"`
	AspectRatio       string `xml:"aspectRatio,attr"`
}

type nrtmAudioFormat struct {
	NumOfChannel  string          `xml:"numOfChannel,attr"`
	AudioRecPorts []nrtmAudioPort `xml:"AudioRecPort"`
}

type nrtmAudioPort struct {
	Port       string `xml:"port,attr"`
	AudioCodec string `xml:"audioCodec,attr"`
	TrackDst   string `xml:"trackDst,attr"`
}

type nrtmDevice struct {
	Manufacturer string `xml:"manufacturer,attr"`
	ModelName    string `xml:"modelName,attr"`
	SerialNo     string `xml:"serialNo,attr"`
}

type nrtmRecMode struct {
	Type     string `xml:"type,attr"`
	CacheRec string `xml:"cacheRec,attr"`
}

type nrtmAcqRecord struct {
	Groups       []nrtmGroup       `xml:"Group"`
	ChangeTables []nrtmChangeTable `xml:"ChangeTable"`
}

type nrtmGroup struct {
	Name  string     `xml:"name,attr"`
	Items []nrtmItem `xml:"Item"`
}

type nrtmItem struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type nrtmChangeTable struct {
	Name   string      `xml:"name,attr"`
	Events []nrtmEvent `xml:"Event"`
}

type nrtmEvent struct {
	FrameCount string `xml:"frameCount,attr"`
	Status     string `xml:"status,attr"`
}

// Ensure capitalizeFirstRune is accessible (avoid unused import).
var _ = fmt.Sprintf
