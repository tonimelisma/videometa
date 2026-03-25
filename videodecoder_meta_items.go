package videometa

import (
	"bytes"
	"math"
	"sort"
)

const maxMetaItemPayload = 10 << 20

type metaItemKind int

const (
	metaItemKindUnknown metaItemKind = iota
	metaItemKindEXIF
	metaItemKindXMP
)

type metaItemContext struct {
	items       map[uint32]*metaItemInfo
	primaryItem uint32
	idatOffset  int64
	idatSize    uint64
	idatData    []byte
}

type metaItemInfo struct {
	itemType           string
	contentType        string
	contentEncoding    string
	protectionIndex    uint16
	constructionMethod uint16
	dataReferenceIndex uint16
	baseOffset         uint64
	extents            []metaItemExtent
}

type metaItemExtent struct {
	index  uint64
	offset uint64
	length uint64
}

func newMetaItemContext() *metaItemContext {
	return &metaItemContext{
		items: make(map[uint32]*metaItemInfo),
	}
}

func (ctx *metaItemContext) item(id uint32) *metaItemInfo {
	item := ctx.items[id]
	if item == nil {
		item = &metaItemInfo{}
		ctx.items[id] = item
	}
	return item
}

func (item *metaItemInfo) kind() metaItemKind {
	switch item.contentType {
	case "Exif":
		return metaItemKindEXIF
	case "application/rdf+xml":
		return metaItemKindXMP
	}

	switch item.itemType {
	case "Exif":
		return metaItemKindEXIF
	}

	return metaItemKindUnknown
}

func (item *metaItemInfo) typeName() string {
	if item.contentType != "" {
		return item.contentType
	}
	if item.itemType != "" {
		return item.itemType
	}
	return "unknown"
}

func (d *videoDecoderMP4) decodeMetaIloc(ilocStart int64, ilocSize uint64, ctx *metaItemContext) {
	version := d.read1()
	_ = d.readBytes(3) // flags

	sizeField := d.read2()
	offsetSize := uint8(sizeField >> 12)
	lengthSize := uint8((sizeField >> 8) & 0x0f)
	baseOffsetSize := uint8((sizeField >> 4) & 0x0f)
	indexSize := uint8(sizeField & 0x0f)

	var itemCount uint32
	if version < 2 {
		itemCount = uint32(d.read2())
	} else {
		itemCount = d.read4()
	}

	for i := uint32(0); i < itemCount; i++ {
		var itemID uint32
		if version < 2 {
			itemID = uint32(d.read2())
		} else {
			itemID = d.read4()
		}

		item := ctx.item(itemID)
		if version == 1 || version == 2 {
			item.constructionMethod = d.read2() & 0x000f
		}

		item.dataReferenceIndex = d.read2()

		baseOffset, ok := d.readSizedUint(baseOffsetSize, 0)
		if !ok {
			d.warnMetaItem("invalid iloc base offset size %d in box at 0x%x", baseOffsetSize, ilocStart)
			return
		}
		item.baseOffset = baseOffset

		extentCount := d.read2()
		item.extents = item.extents[:0]
		for j := uint16(0); j < extentCount; j++ {
			extentIndex := uint64(1)
			if version == 1 || version == 2 {
				var ok bool
				extentIndex, ok = d.readSizedUint(indexSize, 1)
				if !ok {
					d.warnMetaItem("invalid iloc extent index size %d in box at 0x%x", indexSize, ilocStart)
					return
				}
			}

			extentOffset, ok := d.readSizedUint(offsetSize, 0)
			if !ok {
				d.warnMetaItem("invalid iloc extent offset size %d in box at 0x%x", offsetSize, ilocStart)
				return
			}
			extentLength, ok := d.readSizedUint(lengthSize, 0)
			if !ok {
				d.warnMetaItem("invalid iloc extent length size %d in box at 0x%x", lengthSize, ilocStart)
				return
			}

			item.extents = append(item.extents, metaItemExtent{
				index:  extentIndex,
				offset: extentOffset,
				length: extentLength,
			})
		}
	}
}

func (d *videoDecoderMP4) decodeMetaIinf(iinfStart int64, iinfSize uint64, ctx *metaItemContext) {
	version := d.read1()
	_ = d.readBytes(3) // flags
	if version == 0 {
		_ = d.read2() // entry_count
	} else {
		_ = d.read4() // entry_count
	}

	iinfEnd := boxEnd(iinfStart, iinfSize)
	for d.pos() < iinfEnd {
		startPos := d.pos()
		boxSize, boxType, isEOF := d.readBoxHeader()
		if isEOF || boxSize < 8 {
			break
		}

		if boxType.String() == "infe" {
			d.decodeMetaInfe(startPos, boxSize, ctx)
		}

		d.seekToBoxEnd(startPos, boxSize)
	}
}

func (d *videoDecoderMP4) decodeMetaInfe(infeStart int64, infeSize uint64, ctx *metaItemContext) {
	version := d.read1()
	_ = d.readBytes(3) // flags

	var itemID uint32
	var protectionIndex uint16
	var itemType string

	switch version {
	case 0, 1:
		itemID = uint32(d.read2())
		protectionIndex = d.read2()
		item := ctx.item(itemID)
		item.protectionIndex = protectionIndex
		_ = d.readNullString(boxEnd(infeStart, infeSize)) // name
		item.contentType = d.readNullString(boxEnd(infeStart, infeSize))
		item.contentEncoding = d.readNullString(boxEnd(infeStart, infeSize))
	case 2:
		itemID = uint32(d.read2())
		protectionIndex = d.read2()
		itemType = d.readFourCC().String()
		item := ctx.item(itemID)
		item.protectionIndex = protectionIndex
		item.itemType = itemType
		_ = d.readNullString(boxEnd(infeStart, infeSize)) // name
		switch itemType {
		case "mime":
			item.contentType = d.readNullString(boxEnd(infeStart, infeSize))
			item.contentEncoding = d.readNullString(boxEnd(infeStart, infeSize))
		case "uri ":
			_ = d.readNullString(boxEnd(infeStart, infeSize))
		}
	case 3:
		itemID = d.read4()
		protectionIndex = d.read2()
		itemType = d.readFourCC().String()
		item := ctx.item(itemID)
		item.protectionIndex = protectionIndex
		item.itemType = itemType
		_ = d.readNullString(boxEnd(infeStart, infeSize)) // name
		switch itemType {
		case "mime":
			item.contentType = d.readNullString(boxEnd(infeStart, infeSize))
			item.contentEncoding = d.readNullString(boxEnd(infeStart, infeSize))
		case "uri ":
			_ = d.readNullString(boxEnd(infeStart, infeSize))
		}
	default:
		d.warnMetaItem("unsupported infe version %d at 0x%x", version, infeStart)
	}
}

func (d *videoDecoderMP4) decodeMetaPitm(pitmStart int64, _ uint64, ctx *metaItemContext) {
	version := d.read1()
	_ = d.readBytes(3) // flags
	if version == 0 {
		ctx.primaryItem = uint32(d.read2())
		return
	}
	ctx.primaryItem = d.read4()
	_ = pitmStart
}

func (d *videoDecoderMP4) decodeMetaItems(ctx *metaItemContext) {
	if ctx == nil || len(ctx.items) == 0 || !d.opts.Sources.Has(EXIF|XMP) {
		return
	}

	itemIDs := make([]int, 0, len(ctx.items))
	for id := range ctx.items {
		itemIDs = append(itemIDs, int(id))
	}
	sort.Ints(itemIDs)

	for _, rawID := range itemIDs {
		itemID := uint32(rawID)
		item := ctx.items[itemID]
		if item == nil {
			continue
		}

		switch item.kind() {
		case metaItemKindEXIF:
			if !d.opts.Sources.Has(EXIF) {
				continue
			}
		case metaItemKindXMP:
			if !d.opts.Sources.Has(XMP) {
				continue
			}
		default:
			continue
		}

		if item.contentEncoding != "" {
			d.warnMetaItem("can't currently decode %s encoded %s metadata item %d", item.contentEncoding, item.typeName(), itemID)
			continue
		}
		if item.protectionIndex != 0 {
			d.warnMetaItem("can't currently decode protected %s metadata item %d", item.typeName(), itemID)
			continue
		}
		if item.dataReferenceIndex != 0 {
			d.warnMetaItem("can't currently decode external %s metadata item %d", item.typeName(), itemID)
			continue
		}
		if len(item.extents) == 0 {
			d.warnMetaItem("no extents for %s metadata item %d", item.typeName(), itemID)
			continue
		}
		if item.constructionMethod > 1 {
			d.warnMetaItem("can't currently extract %s item %d with construction method %d", item.typeName(), itemID, item.constructionMethod)
			continue
		}
		if item.constructionMethod == 0 && !d.canSeek {
			d.warnMetaItem("can't currently extract file-offset %s item %d from a non-seekable reader", item.typeName(), itemID)
			continue
		}
		if item.constructionMethod == 1 && ctx.idatSize == 0 && len(ctx.idatData) == 0 {
			d.warnMetaItem("missing idat for %s item %d with construction method 1", item.typeName(), itemID)
			continue
		}
		if item.constructionMethod == 1 && !d.canSeek && len(ctx.idatData) == 0 {
			d.warnMetaItem("can't currently extract idat-backed %s item %d from a non-seekable reader without buffered idat data", item.typeName(), itemID)
			continue
		}

		var (
			data []byte
			ok   bool
		)
		if d.canSeek {
			d.preservePos(func() {
				data, ok = d.readMetaItemPayload(ctx, item)
			})
		} else {
			data, ok = d.readMetaItemPayload(ctx, item)
		}
		if !ok || len(data) == 0 {
			continue
		}

		switch item.kind() {
		case metaItemKindEXIF:
			d.decodeEXIFFromMetaItem(data)
		case metaItemKindXMP:
			_ = d.decodeXMP(bytes.NewReader(data))
		}
	}
}

func (d *videoDecoderMP4) readMetaItemPayload(ctx *metaItemContext, item *metaItemInfo) ([]byte, bool) {
	totalLength := uint64(0)
	for _, extent := range item.extents {
		if extent.length > maxMetaItemPayload || totalLength > maxMetaItemPayload-extent.length {
			d.warnMetaItem("skipping %s item larger than %d bytes", item.typeName(), maxMetaItemPayload)
			return nil, false
		}
		totalLength += extent.length
	}

	data := make([]byte, 0, int(totalLength))
	for _, extent := range item.extents {
		chunk, ok := d.readMetaItemExtent(ctx, item, extent)
		if !ok {
			return nil, false
		}
		data = append(data, chunk...)
	}

	return data, true
}

func (d *videoDecoderMP4) readMetaItemExtent(ctx *metaItemContext, item *metaItemInfo, extent metaItemExtent) ([]byte, bool) {
	if item.constructionMethod == 1 {
		relativeStart, relativeEnd, absoluteStart, ok := d.idatExtentRange(ctx, item, extent)
		if !ok {
			return nil, false
		}
		if len(ctx.idatData) > 0 {
			if relativeEnd > uint64(len(ctx.idatData)) {
				d.warnMetaItem("idat extent for %s item exceeds available idat data", item.typeName())
				return nil, false
			}
			return append([]byte(nil), ctx.idatData[relativeStart:relativeEnd]...), true
		}
		if !d.canSeek {
			d.warnMetaItem("can't currently extract idat-backed %s item from a non-seekable reader without buffered idat data", item.typeName())
			return nil, false
		}

		d.seek(int64(absoluteStart))
		return d.readBytes(int(extent.length)), true
	}

	start, _, ok := d.metaItemExtentRange(item.typeName(), item.baseOffset, extent.offset, extent.length)
	if !ok {
		return nil, false
	}

	d.seek(int64(start))
	return d.readBytes(int(extent.length)), true
}

func (d *videoDecoderMP4) metaItemExtentRange(typeName string, baseOffset uint64, extentOffset uint64, extentLength uint64) (start uint64, end uint64, ok bool) {
	if baseOffset > math.MaxUint64-extentOffset {
		d.warnMetaItem("invalid extent range for %s item", typeName)
		return 0, 0, false
	}
	start = baseOffset + extentOffset
	if start > math.MaxUint64-extentLength {
		d.warnMetaItem("invalid extent range for %s item", typeName)
		return 0, 0, false
	}
	end = start + extentLength
	if end > math.MaxInt64 {
		d.warnMetaItem("invalid extent range for %s item", typeName)
		return 0, 0, false
	}
	return start, end, true
}

func (d *videoDecoderMP4) idatExtentRange(ctx *metaItemContext, item *metaItemInfo, extent metaItemExtent) (relativeStart uint64, relativeEnd uint64, absoluteStart uint64, ok bool) {
	relativeStart, relativeEnd, ok = d.metaItemExtentRange(item.typeName(), item.baseOffset, extent.offset, extent.length)
	if !ok {
		return 0, 0, 0, false
	}
	if relativeEnd > ctx.idatSize {
		d.warnMetaItem("idat extent for %s item exceeds idat box bounds", item.typeName())
		return 0, 0, 0, false
	}
	if ctx.idatOffset < 0 {
		d.warnMetaItem("invalid extent range for %s item", item.typeName())
		return 0, 0, 0, false
	}
	if uint64(ctx.idatOffset) > math.MaxUint64-relativeStart {
		d.warnMetaItem("invalid extent range for %s item", item.typeName())
		return 0, 0, 0, false
	}
	absoluteStart = uint64(ctx.idatOffset) + relativeStart
	if absoluteStart > math.MaxInt64 {
		d.warnMetaItem("invalid extent range for %s item", item.typeName())
		return 0, 0, 0, false
	}
	return relativeStart, relativeEnd, absoluteStart, true
}

func (d *videoDecoderMP4) decodeEXIFFromMetaItem(data []byte) {
	if len(data) == 0 {
		return
	}

	start := 0
	switch {
	case len(data) >= 4 && isTIFFHeader(data):
		d.warnMetaItem("missing Exif header in iloc item payload")
	case len(data) >= 6 && bytes.Equal(data[:6], []byte("Exif\x00\x00")):
		d.warnMetaItem("missing Exif header size in iloc item payload")
		start = 6
	default:
		if len(data) < 4 {
			d.warnMetaItem("invalid EXIF iloc item payload")
			return
		}
		headerSize := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		start = 4 + int(headerSize)
		if start > len(data) {
			d.warnMetaItem("invalid EXIF item header length %d", headerSize)
			return
		}
	}

	if start >= len(data) {
		return
	}
	d.decodeEXIF(bytes.NewReader(data[start:]))
}

func isTIFFHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return bytes.Equal(data[:4], []byte{'M', 'M', 0x00, 0x2a}) ||
		bytes.Equal(data[:4], []byte{'I', 'I', 0x2a, 0x00})
}

func (d *videoDecoderMP4) readNullString(limit int64) string {
	if d.pos() >= limit {
		return ""
	}

	buf := make([]byte, 0, 64)
	for d.pos() < limit {
		b := d.read1()
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}

	return printableString(string(buf))
}

func (d *videoDecoderMP4) readSizedUint(size uint8, defaultValue uint64) (uint64, bool) {
	if size == 0 {
		return defaultValue, true
	}
	if size > 8 {
		return 0, false
	}

	var value uint64
	for i := uint8(0); i < size; i++ {
		value = (value << 8) | uint64(d.read1())
	}
	return value, true
}

func (d *videoDecoderMP4) warnMetaItem(format string, args ...any) {
	if d.opts.Warnf != nil {
		d.opts.Warnf("decode meta item: "+format, args...)
	}
}
