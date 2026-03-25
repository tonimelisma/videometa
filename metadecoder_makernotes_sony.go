package videometa

func (d *videoDecoderMP4) decodeSonyMakerNotes(data []byte, ctx makerNoteContext) {
	start := int64(0)
	if hasSonyMakerNoteHeader(data) {
		start = 12
	}
	if int(start) >= len(data) {
		return
	}

	md := newMakerNoteDecoder(data, ctx.byteOrder, 0)
	md.walkIFD(start, func(tagID, _ uint16, _ uint32, value any) {
		switch tagID {
		case 0x0102:
			if v, ok := value.(uint32); ok {
				d.emitMakerNotesTag("Quality", v)
			}
		case 0x0104:
			d.emitMakerNotesTag("FlashExposureComp", value)
		case 0x0105:
			if v, ok := value.(uint32); ok {
				d.emitMakerNotesTag("Teleconverter", v)
			}
		case 0x0112:
			if v, ok := value.(int32); ok {
				d.emitMakerNotesTag("WhiteBalanceFineTune", v)
			}
		case 0x0115:
			if v, ok := value.(uint32); ok {
				d.emitMakerNotesTag("WhiteBalance", v)
			}
		}
	})
}
