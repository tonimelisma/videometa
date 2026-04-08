package videometa

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type goProGPMFRecord struct {
	tag     string
	typ     byte
	size    int
	count   int
	payload []byte
}

func (d *videoDecoderMP4) decodeGoProStringBox(namespace, tag string, payloadLen int) {
	if payloadLen <= 0 {
		return
	}

	value := sanitizeMetadataString(string(trimTrailingNulls(d.readBytes(payloadLen))))
	if value == "" {
		return
	}
	d.emitVendorTag(namespace, tag, value)
}

func (d *videoDecoderMP4) decodeGoProHexBox(namespace, tag string, payloadLen int) {
	if payloadLen <= 0 {
		return
	}

	value := strings.ToLower(fmt.Sprintf("%x", d.readBytes(payloadLen)))
	if value == "" {
		return
	}
	d.emitVendorTag(namespace, tag, value)
}

func (d *videoDecoderMP4) decodeGoProGPMF(data []byte) {
	d.decodeGoProGPMFContainer("GoPro/moov/udta/GPMF", data, "")
}

func (d *videoDecoderMP4) decodeGoProGPMFContainer(namespace string, data []byte, deviceID string) {
	for offset := 0; offset+8 <= len(data); {
		record, recordLen, ok := nextGoProGPMFRecord(data[offset:])
		if !ok {
			break
		}

		switch {
		case record.typ == 0 && record.tag == "DEVC":
			d.decodeGoProDeviceContainer(namespace, record.payload)
		case record.typ == 0:
			d.decodeGoProGPMFContainer(namespace, record.payload, deviceID)
		default:
			d.emitGoProGPMFTag(namespace, deviceID, record)
		}

		offset += recordLen
	}
}

func (d *videoDecoderMP4) decodeGoProDeviceContainer(namespace string, data []byte) {
	deviceID := ""

	for offset := 0; offset+8 <= len(data); {
		record, recordLen, ok := nextGoProGPMFRecord(data[offset:])
		if !ok {
			break
		}

		switch {
		case record.typ == 0:
			d.decodeGoProGPMFContainer(namespace, record.payload, deviceID)
		case record.tag == "DVID":
			deviceID = decodeGoProDeviceID(record)
		default:
			d.emitGoProGPMFTag(namespace, deviceID, record)
		}

		offset += recordLen
	}
}

func (d *videoDecoderMP4) emitGoProGPMFTag(namespace, deviceID string, record goProGPMFRecord) {
	tagName := ""
	var value any
	emit := false

	switch record.tag {
	case "VERS":
		tagName = "MetadataVersion"
		value = strings.Join(goProUint8Strings(record.payload), " ")
		emit = true
	case "FMWR":
		tagName = "FirmwareVersion"
		value = goProCString(record.payload)
		emit = true
	case "CASN":
		tagName = "CameraSerialNumber"
		value = goProCString(record.payload)
		emit = true
	case "MINF":
		tagName = "Model"
		value = goProCString(record.payload)
		emit = true
	case "MUID":
		tagName = "MediaUniqueID"
		value = strings.Join(goProUint32Strings(record.payload), " ")
		emit = true
	case "CPIN":
		tagName = "ChapterNumber"
		value = int(goProUint32(record.payload))
		emit = true
	case "HDRV":
		tagName = "HDRVideo"
		value = goProCString(record.payload)
		emit = true
	case "OREN":
		tagName = "AutoRotation"
		value = goProCString(record.payload)
		emit = true
	case "DZOM":
		tagName = "DigitalZoomOn"
		value = goProCString(record.payload)
		emit = true
	case "DZST":
		tagName = "DigitalZoom"
		value = int(goProUint32(record.payload))
		emit = true
	case "SMTR":
		tagName = "SpotMeter"
		value = goProCString(record.payload)
		emit = true
	case "PRTN":
		tagName = "Protune"
		value = goProCString(record.payload)
		emit = true
	case "PTWB":
		tagName = "WhiteBalance"
		value = goProCString(record.payload)
		emit = true
	case "PTSH":
		tagName = "Sharpness"
		value = goProCString(record.payload)
		emit = true
	case "PTCL":
		tagName = "ColorMode"
		value = goProCString(record.payload)
		emit = true
	case "EXPT":
		tagName = "ExposureType"
		value = goProCString(record.payload)
		emit = true
	case "PIMX":
		tagName = "AutoISOMax"
		value = int(goProUint32(record.payload))
		emit = true
	case "PIMN":
		tagName = "AutoISOMin"
		value = int(goProUint32(record.payload))
		emit = true
	case "PTEV":
		tagName = "ExposureCompensation"
		value = goProStringNumber(goProCString(record.payload))
		emit = true
	case "RATE":
		tagName = "Rate"
		value = goProCString(record.payload)
		emit = true
	case "SROT":
		tagName = "SensorReadoutTime"
		value = goProFirstFloat(record.payload)
		emit = true
	case "EISE":
		tagName = "ElectronicStabilizationOn"
		value = goProCString(record.payload)
		emit = true
	case "EISA":
		tagName = "ElectronicImageStabilization"
		value = goProCString(record.payload)
		emit = true
	case "HCTL":
		tagName = "HorizonControl"
		value = goProCString(record.payload)
		emit = true
	case "AUPT":
		tagName = "AutoProtune"
		value = goProCString(record.payload)
		emit = true
	case "APTO":
		tagName = "AudioProtuneOption"
		value = goProCString(record.payload)
		emit = true
	case "AUDO":
		tagName = "AudioSetting"
		value = goProCString(record.payload)
		emit = true
	case "AUBT":
		tagName = "AudioBlueTooth"
		value = goProCString(record.payload)
		emit = true
	case "PRJT":
		tagName = "LensProjection"
		value = goProFourCC(record.payload)
		emit = true
	case "CDAT":
		tagName = "CreationDate"
		value = time.Unix(int64(goProUint64(record.payload)), 0).UTC()
		emit = true
	case "SCTM":
		tagName = "ScheduleCaptureTime"
		value = int(goProUint32(record.payload))
		emit = true
	case "SCAP":
		tagName = "ScheduleCapture"
		value = goProCString(record.payload)
		emit = true
	case "CDTM":
		tagName = "CaptureDelayTimer"
		value = int(goProUint32(record.payload))
		emit = true
	case "DUST":
		tagName = "DurationSetting"
		value = goProCString(record.payload)
		emit = true
	case "VRES":
		tagName = "VideoFrameSize"
		values := goProUint32Values(record.payload)
		if len(values) >= 2 {
			value = fmt.Sprintf("%d %d", values[0], values[1])
			emit = true
		}
	case "VFPS":
		tagName = "VideoFrameRate"
		value = strings.Join(goProUint32Strings(record.payload), " ")
		emit = true
	case "HSGT":
		tagName = "HindsightSettings"
		value = goProCString(record.payload)
		emit = true
	case "BITR":
		tagName = "BitrateSetting"
		value = goProCString(record.payload)
		emit = true
	case "MMOD":
		tagName = "MediaMode"
		value = goProCString(record.payload)
		emit = true
	case "RAMP":
		tagName = "SpeedRampSetting"
		value = goProCString(record.payload)
		emit = true
	case "TZON":
		tagName = "TimeZone"
		value = int(goProInt16(record.payload))
		emit = true
	case "DZMX":
		tagName = "DigitalZoomAmount"
		value = goProFirstFloat(record.payload)
		emit = true
	case "CTRL":
		tagName = "ControlLevel"
		value = goProCString(record.payload)
		emit = true
	case "PWPR":
		tagName = "PowerProfile"
		value = goProCString(record.payload)
		emit = true
	case "ORDP":
		tagName = "OrientationDataPresent"
		value = goProCString(record.payload)
		emit = true
	case "CLDP":
		tagName = "ClassificationDataPresent"
		value = goProCString(record.payload)
		emit = true
	case "PIMD":
		tagName = "ProtuneISOMode"
		value = goProCString(record.payload)
		emit = true
	case "ABSC":
		tagName = "AutoBoostScore"
		value = goProFirstFloat(record.payload)
		emit = true
	case "ZFOV":
		tagName = "DiagonalFieldOfView"
		value = goProFirstFloat(record.payload)
		emit = true
	case "VFOV":
		tagName = "FieldOfView"
		value = goProCString(record.payload)
		emit = true
	case "MXCF":
		tagName = "MappingXMode"
		value = goProCString(record.payload)
		emit = true
	case "MAPX":
		tagName = "MappingXCoefficients"
		value = goProFirstFloat(record.payload)
		emit = true
	case "MYCF":
		tagName = "MappingYMode"
		value = goProCString(record.payload)
		emit = true
	case "MAPY":
		tagName = "MappingYCoefficients"
		value = goProFirstFloat(record.payload)
		emit = true
	case "PYCF":
		tagName = "PolynomialPower"
		value = goProStringChunks(record.payload, 8)
		emit = true
	case "POLY":
		tagName = "PolynomialCoefficients"
		value = goProFloatListString(record.payload)
		emit = true
	case "ZMPL":
		tagName = "ZoomScaleNormalization"
		value = goProFirstFloat(record.payload)
		emit = true
	case "ARUW":
		tagName = "AspectRatioUnwarped"
		value = goProFirstFloat(record.payload)
		emit = true
	case "ARWA":
		tagName = "AspectRatioWarped"
		value = goProFirstFloat(record.payload)
		emit = true
	case "DVNM":
		_ = deviceID
		tagName = "DeviceName"
		value = goProCString(record.payload)
		emit = true
	}

	if !emit {
		return
	}

	d.emitVendorTag(namespace, tagName, value)
}

func nextGoProGPMFRecord(data []byte) (goProGPMFRecord, int, bool) {
	if len(data) < 8 {
		return goProGPMFRecord{}, 0, false
	}

	tagBytes := data[:4]
	typ := data[4]
	size := int(data[5])
	count := int(goProUint16(data[6:8]))
	if size < 0 || count < 0 {
		return goProGPMFRecord{}, 0, false
	}
	if tagBytes[0] == 0 && tagBytes[1] == 0 && tagBytes[2] == 0 && tagBytes[3] == 0 &&
		typ == 0 && size == 0 && count == 0 {
		return goProGPMFRecord{}, 0, false
	}

	payloadLen := size * count
	if payloadLen < 0 || payloadLen > len(data)-8 {
		return goProGPMFRecord{}, 0, false
	}
	paddedLen := (payloadLen + 3) &^ 3
	totalLen := 8 + paddedLen
	if totalLen > len(data) {
		return goProGPMFRecord{}, 0, false
	}

	return goProGPMFRecord{
		tag:     string(tagBytes),
		typ:     typ,
		size:    size,
		count:   count,
		payload: data[8 : 8+payloadLen],
	}, totalLen, true
}

func decodeGoProDeviceID(record goProGPMFRecord) string {
	switch record.typ {
	case 'F':
		return goProFourCC(record.payload)
	case 'L':
		return strconv.Itoa(int(goProUint32(record.payload)))
	default:
		return goProCString(record.payload)
	}
}

func goProCString(data []byte) string {
	return sanitizeMetadataString(string(trimTrailingNulls(data)))
}

func goProFourCC(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	return sanitizeMetadataString(string(data[:4]))
}

func goProStringChunks(data []byte, chunkSize int) []string {
	if chunkSize <= 0 {
		return nil
	}

	values := make([]string, 0, len(data)/chunkSize)
	for offset := 0; offset+chunkSize <= len(data); offset += chunkSize {
		values = append(values, goProCString(data[offset:offset+chunkSize]))
	}
	return values
}

func goProUint8Strings(data []byte) []string {
	values := make([]string, 0, len(data))
	for _, value := range data {
		values = append(values, strconv.Itoa(int(value)))
	}
	return values
}

func goProUint32(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
}

func goProUint64(data []byte) uint64 {
	if len(data) < 8 {
		return 0
	}
	return uint64(goProUint32(data[:4]))<<32 | uint64(goProUint32(data[4:8]))
}

func goProInt16(data []byte) int16 {
	if len(data) < 2 {
		return 0
	}
	return int16(uint16(data[0])<<8 | uint16(data[1]))
}

func goProUint16(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	return uint16(data[0])<<8 | uint16(data[1])
}

func goProUint32Values(data []byte) []uint32 {
	values := make([]uint32, 0, len(data)/4)
	for offset := 0; offset+4 <= len(data); offset += 4 {
		values = append(values, goProUint32(data[offset:offset+4]))
	}
	return values
}

func goProUint32Strings(data []byte) []string {
	values := goProUint32Values(data)
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strconv.FormatUint(uint64(value), 10))
	}
	return result
}

func goProFirstFloat(data []byte) float64 {
	values := goProFloatValues(data)
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func goProFloatValues(data []byte) []float64 {
	values := make([]float64, 0, len(data)/4)
	for offset := 0; offset+4 <= len(data); offset += 4 {
		bits := goProUint32(data[offset : offset+4])
		values = append(values, float64(math.Float32frombits(bits)))
	}
	return values
}

func goProFloatListString(data []byte) string {
	values := goProFloatValues(data)
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(value, 'g', 15, 64))
	}
	return strings.Join(parts, " ")
}

func goProStringNumber(value string) any {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}
