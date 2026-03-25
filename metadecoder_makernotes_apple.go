package videometa

var appleMakerNoteTags = map[uint16]string{
	0x0001: "MakerNoteVersion",
	0x0004: "AEStable",
	0x0005: "AETarget",
	0x0006: "AEAverage",
	0x0007: "AFStable",
	0x0008: "AccelerationVector",
	0x000A: "HDRImageType",
	0x000B: "BurstUUID",
	0x000C: "FocusDistanceRange",
	0x000F: "OISMode",
	0x0011: "ContentIdentifier",
	0x0014: "ImageCaptureType",
	0x0015: "ImageUniqueID",
	0x0017: "LivePhotoVideoIndex",
	0x0019: "ImageProcessingFlags",
	0x001A: "QualityHint",
	0x001D: "LuminanceNoiseAmplitude",
	0x001F: "PhotosAppFeatureFlags",
	0x0021: "HDRHeadroom",
	0x0025: "SceneFlags",
	0x0026: "SignalToNoiseRatioType",
	0x0027: "SignalToNoiseRatio",
	0x002B: "PhotoIdentifier",
	0x002D: "ColorTemperature",
}

func (d *videoDecoderMP4) decodeAppleMakerNotes(data []byte, ctx makerNoteContext) {
	if len(data) <= 14 {
		return
	}

	md := newMakerNoteDecoder(data, ctx.byteOrder, 0)
	md.walkIFD(14, func(tagID, _ uint16, _ uint32, value any) {
		tagName := appleMakerNoteTags[tagID]
		if tagName == "" {
			return
		}
		d.emitMakerNotesTag(tagName, value)
	})
}
