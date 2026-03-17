# videometa Architecture

Each design element has an ID (`ARCH-*`) linked back to requirements (`REQ-*`).

---

## 1. Data Flow (`ARCH-FLOW-*`)

| ID | Description | Traces to |
|----|-------------|-----------|
| ARCH-FLOW-01 | End-to-end pipeline from reader through box parser to metadata decoders and callbacks | REQ-API-01..18 |

```
io.ReadSeeker (or io.Reader fallback)
  → streamReader (binary I/O layer)
    → ISOBMFF box parser (videodecoder_mp4.go)
      → metadata router (box path → decoder dispatch)
        → EXIF decoder → HandleTag callback
        → XMP decoder → HandleTag callback
        → IPTC decoder → HandleTag callback
        → QuickTime decoder → HandleTag callback
        → CONFIG extractor → DecodeResult.VideoConfig
```

---

## 2. File Layout (`ARCH-FILE-*`)

| ID | File | Purpose | Traces to |
|----|------|---------|-----------|
| ARCH-FILE-01 | `videometa.go` | Public API: Decode, DecodeAll, Options, TagInfo, Tags, Source, VideoFormat, DecodeResult, VideoConfig | REQ-API-* |
| ARCH-FILE-02 | `io.go` | streamReader: binary reads, byte-order, seek, buffer pool, panic control flow | REQ-NF-01, REQ-NF-02 |
| ARCH-FILE-03 | `videodecoder_mp4.go` | ISOBMFF box parser + metadata routing | REQ-BOX-* |
| ARCH-FILE-04 | `metadecoder_exif.go` | EXIF IFD parser | REQ-EXIF-01..06 |
| ARCH-FILE-05 | `metadecoder_exif_fields.go` | EXIF tag name table (~200 tags) | REQ-EXIF-04 |
| ARCH-FILE-06 | `metadecoder_xmp.go` | XMP/RDF XML parser | REQ-XMP-* |
| ARCH-FILE-07 | `metadecoder_iptc.go` | IPTC record parser | REQ-IPTC-* |
| ARCH-FILE-08 | `metadecoder_iptc_fields.json` | IPTC field definitions (embedded) | REQ-IPTC-01 |
| ARCH-FILE-09 | `metadecoder_quicktime.go` | QuickTime ilst/freeform parser, mvhd/tkhd | REQ-QT-* |
| ARCH-FILE-10 | `metadecoder_quicktime_fields.go` | QuickTime tag name table | REQ-QT-07 |
| ARCH-FILE-11 | `metadecoder_makernotes_apple.go` | Apple MakerNotes | REQ-EXIF-07 |
| ARCH-FILE-12 | `metadecoder_makernotes_canon.go` | Canon MakerNotes | REQ-EXIF-08 |
| ARCH-FILE-13 | `metadecoder_makernotes_sony.go` | Sony MakerNotes | REQ-EXIF-09 |
| ARCH-FILE-14 | `helpers.go` | Rat[T], InvalidFormatError, value converters, ISO6709 parser | REQ-QT-06, REQ-NF-06 |
| ARCH-FILE-15 | `gen/main.go` | Golden file generator (runs exiftool) | REQ-NF-04 |
| ARCH-FILE-16 | `testdata/` | Test video files + golden JSON | REQ-TEST-* |
| ARCH-FILE-17 | `.github/workflows/ci.yml` | CI with exiftool validation | REQ-NF-10 |

---

## 3. ISOBMFF Box Parser (`ARCH-BOX-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|-----------|
| ARCH-BOX-01 | Iterative traversal with explicit path stack (not recursive) | Prevents stack overflow on pathological files | REQ-BOX-01..05, REQ-NF-06 |
| ARCH-BOX-02 | `readBoxHeader()` returns (offset, totalSize, fourcc, isFullBox) | Core primitive for all box navigation | REQ-BOX-01..04 |
| ARCH-BOX-03 | Skip mdat by seeking (ReadSeeker) or read+discard (Reader) | mdat can be gigabytes | REQ-BOX-05, REQ-API-03 |
| ARCH-BOX-04 | Routing table: box path → decoder function | Extensible, easy to add new box handlers | REQ-BOX-07, REQ-BOX-08 |
| ARCH-BOX-05 | ftyp validation: check major brand and compatible brands | Detect MOV vs MP4 internally | REQ-BOX-06, REQ-API-04 |

### Box Path Routing Table

| Box path | Action |
|----------|--------|
| `ftyp` | Validate brand, set MOV/MP4 mode |
| `moov/mvhd` | → CONFIG: timestamps, duration, timescale |
| `moov/trak/tkhd` | → CONFIG: dimensions, rotation |
| `moov/trak/mdia/minf/stbl/stsd` | → CONFIG: codec fourcc + params |
| `moov/udta/meta` | Parse as FullBox, descend |
| `moov/udta/meta/ilst` | → QuickTime decoder |
| `moov/udta/meta/ilst/----` | → QuickTime freeform decoder |
| `uuid (XMP GUID)` | → XMP decoder |
| `uuid (EXIF GUID)` or `moov/meta` item | → EXIF decoder |
| `moof` | → Return "fragmented MP4 not supported" error |
| Other | Skip (warn if potential metadata) |

---

## 4. Metadata Decoders (`ARCH-DEC-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|-----------|
| ARCH-DEC-01 | Internal `decoder` interface: `decode() error` | Same pattern as imagemeta | REQ-API-01 |
| ARCH-DEC-02 | EXIF decoder: reimplemented from TIFF/EXIF spec, validated against exiftool | D-21 | REQ-EXIF-01..06 |
| ARCH-DEC-03 | XMP decoder: `encoding/xml` based RDF parser | Same approach as imagemeta | REQ-XMP-* |
| ARCH-DEC-04 | IPTC decoder: binary record parser with embedded JSON field defs | Same as imagemeta | REQ-IPTC-* |
| ARCH-DEC-05 | QuickTime decoder: new, ilst iteration + freeform atom parsing | No imagemeta equivalent | REQ-QT-* |
| ARCH-DEC-06 | Value converters ported from exiftool's Perl logic | exiftool is reference implementation | REQ-EXIF-05 |
| ARCH-DEC-07 | Tag names match exiftool output exactly | D-09 | REQ-QT-07 |
| ARCH-DEC-08 | MakerNotes: per-manufacturer Go files with embedded tag tables | Clean separation, easy to add more manufacturers | REQ-EXIF-07..09 |

### Single Package Design

All code lives in one `videometa` package (no subpackages), following imagemeta's pattern. This keeps the API simple and avoids import cycles.

---

## 5. streamReader (`ARCH-IO-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|-----------|
| ARCH-IO-01 | Wraps io.ReadSeeker with convenience binary reads (read1/2/4/8) | Reduces boilerplate in decoders | REQ-NF-01 |
| ARCH-IO-02 | Byte-order-aware (big/little endian switching) | EXIF can be either; ISOBMFF is always big-endian | REQ-EXIF-02 |
| ARCH-IO-03 | Panic-based control flow (panic(errStop) on EOF) | Avoids error-check on every binary read; recovered at Decode() boundary | REQ-NF-01 |
| ARCH-IO-04 | Buffer pool via sync.Pool | Reduces allocations for performance | REQ-NF-02 |
| ARCH-IO-05 | io.Reader fallback: wrap in buffered reader, track position, discard-seek | D-10 | REQ-API-03 |

---

## 6. Error Handling (`ARCH-ERR-*`)

| ID | Design element | Traces to |
|----|----------------|-----------|
| ARCH-ERR-01 | `InvalidFormatError` for malformed input (fuzz-safe) | REQ-NF-06 |
| ARCH-ERR-02 | `ErrStopWalking` for caller-initiated early termination | REQ-API-15 |
| ARCH-ERR-03 | Internal `errStop` for EOF handling (panic/recover) | REQ-NF-01 |
| ARCH-ERR-04 | Timeout via goroutine + channel (same as imagemeta) | REQ-API-10 |
| ARCH-ERR-05 | Partial failure: EXIF decode error doesn't prevent XMP extraction | REQ-API-17, REQ-API-18 |

---

## 7. Testing Architecture (`ARCH-TEST-*`)

| ID | Design element | Traces to |
|----|----------------|-----------|
| ARCH-TEST-01 | `go generate ./gen` runs exiftool on test videos, saves JSON to testdata/ | REQ-NF-04 |
| ARCH-TEST-02 | Tests compare videometa output against committed golden JSON | REQ-NF-04 |
| ARCH-TEST-03 | CI runs exiftool live and diffs against committed golden files | REQ-NF-10 |
| ARCH-TEST-04 | Normalization rules for comparison: float precision, string trimming, type coercion | REQ-NF-04 |
| ARCH-TEST-05 | One fuzz target per decoder path, seed corpus from test videos | REQ-NF-05 |
| ARCH-TEST-06 | Benchmarks: per-source, all-sources, per-file-type | REQ-NF-03 |

---

## 8. Dependencies (`ARCH-DEP-*`)

| ID | Dependency | Type | Purpose | Traces to |
|----|------------|------|---------|-----------|
| ARCH-DEP-01 | golang.org/x/text | Runtime | IPTC character set decoding (ISO-8859-1) | REQ-NF-08 |
| ARCH-DEP-02 | frankban/quicktest | Test | Test assertions (following imagemeta) | — |
| ARCH-DEP-03 | google/go-cmp | Test | Deep comparison (following imagemeta) | — |

---

## 9. Key Design Decisions Summary

| Decision | Rationale |
|----------|-----------|
| Single package, no subpackages | Follows imagemeta; simpler API |
| Iterative box traversal (ARCH-BOX-01) | Prevents stack overflow on pathological files |
| Seek past mdat (ARCH-BOX-03) | mdat is AV data, can be gigabytes |
| Panic-based control flow (ARCH-IO-03) | Readable decoder code; recovered at boundary |
| exiftool as reference, not imagemeta (ARCH-DEC-06) | More complete edge case handling |
| Per-manufacturer MakerNotes Go files (ARCH-DEC-08) | Clean separation, easy to add more manufacturers |
