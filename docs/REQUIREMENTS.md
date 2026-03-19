# videometa Requirements

Module: `github.com/tonimelisma/videometa`

Every requirement has a unique ID (`REQ-*`) for traceability to architecture (`ARCH-*`), source files, and tests.

## Decisions Registry

| ID | Decision | Choice |
|----|----------|--------|
| D-01 | Runtime dependencies | Zero (except golang.org/x/text for IPTC charset) |
| D-02 | Tag overlap handling | Report all tags separately (like exiftool); convenience methods deduplicate with priority |
| D-03 | VideoFormat enum | Single MP4 value; auto-detect MOV vs MP4 internally from ftyp brand |
| D-04 | Format detection | Optional — auto-detect from ftyp if not specified |
| D-05 | EXIF tag scope | Match imagemeta's set (~200 tags) |
| D-06 | MakerNotes scope | Apple, Canon, Sony; embedded Go tag tables per manufacturer |
| D-07 | MakerNotes timing | v1 scope, late milestone |
| D-08 | Test system | Committed golden JSON + CI validation via GitHub Actions + exiftool |
| D-09 | QuickTime tag names | Match exiftool exactly |
| D-10 | Large file handling | Prefer io.ReadSeeker; io.Reader fallback (read+discard for mdat) |
| D-11 | Unknown boxes | Skip silently; warn only for metadata-bearing boxes (e.g., unrecognized UUID) |
| D-12 | GPS coordinates | Decimal degrees float64 (matching exiftool -n) |
| D-13 | Codec info | Extract codec fourcc + basic params via CONFIG source |
| D-14 | Fragmented MP4 | Not supported in v1 (return error if detected) |
| D-15 | Thumbnails/cover art | Out of scope for v1 |
| D-16 | Timestamp type | Go time.Time; preserve original timezone; GetDateTimeUTC() convenience |
| D-17 | Partial reads | Best-effort mode — succeed if moov at start, return partial + error if not |
| D-18 | XMP extensions | Main packet only (skip extended XMP) |
| D-19 | Convenience API | HandleTag callback + DecodeAll() returning Tags struct |
| D-20 | No-metadata files | Return empty result, no error |
| D-21 | Decoder approach | Implement from specs; exiftool logic as reference; can copy imagemeta code we authored |
| D-22 | Traceability | REQ-* → ARCH-* → source file → test (bidirectional) |

---

## 1. Scope Statement

- Read-only metadata extraction from MP4/MOV (ISOBMFF containers)
- Pure Go, no CGo, no external binaries
- v1: MP4/MOV only (AVI/MKV deferred)
- No writing, no transcoding, no thumbnails (D-15), no fragmented MP4 (D-14)

---

## 2. API Requirements (`REQ-API-*`)

| ID | Requirement | Decision ref |
|----|-------------|-------------|
| REQ-API-01 | `Decode(Options) (DecodeResult, error)` entry point | — |
| REQ-API-02 | `DecodeAll(Options) (Tags, error)` convenience wrapper | D-19 |
| REQ-API-03 | `Options.R` accepts `io.ReadSeeker`; `io.Reader` fallback with degraded performance | D-10 |
| REQ-API-04 | `Options.VideoFormat` optional; auto-detect from ftyp if omitted | D-03, D-04 |
| REQ-API-05 | `Source` bitmask: `EXIF \| XMP \| IPTC \| QUICKTIME \| CONFIG` | — |
| REQ-API-06 | `HandleTag` callback receives `TagInfo{Source, Tag, Namespace, Value}` | — |
| REQ-API-07 | `ShouldHandleTag` filter, `LimitNumTags`, `LimitTagSize` | — |
| REQ-API-08 | `HandleXMP` escape hatch for custom XMP processing | — |
| REQ-API-09 | `Warnf` callback for warnings | — |
| REQ-API-10 | `Timeout` for decode operations | — |
| REQ-API-11 | `Tags.GetDateTime()` returns time.Time with original timezone preserved | D-16 |
| REQ-API-12 | `Tags.GetDateTimeUTC()` normalizes to UTC | D-16 |
| REQ-API-13 | `Tags.GetLatLong()` returns (lat, lon float64) in decimal degrees | D-12 |
| REQ-API-14 | `DecodeResult.VideoConfig` with Width, Height, Duration, Rotation, Codec | D-13 |
| REQ-API-15 | `ErrStopWalking` sentinel for early termination | — |
| REQ-API-16 | All tags reported separately by source (no dedup); convenience methods apply priority | D-02 |
| REQ-API-17 | Empty result (no error) when file has no metadata | D-20 |
| REQ-API-18 | Best-effort partial mode for files where moov position is unknown | D-17 |

---

## 3. Metadata Source Requirements

### ISOBMFF Navigation (`REQ-BOX-*`)

| ID | Requirement |
|----|-------------|
| REQ-BOX-01 | Parse standard box headers (4-byte size + 4-byte fourcc) |
| REQ-BOX-02 | Support 64-bit extended box sizes (size=1) |
| REQ-BOX-03 | Support box extending to EOF (size=0) |
| REQ-BOX-04 | Parse FullBox (version byte + 3-byte flags) for meta, mvhd, tkhd, etc. |
| REQ-BOX-05 | Handle moov at end of file (seek past mdat using declared size) |
| REQ-BOX-06 | Validate ftyp brand for MP4/MOV compatibility |
| REQ-BOX-07 | Skip unknown boxes silently; warn for unrecognized metadata-bearing boxes (D-11) |
| REQ-BOX-08 | Return error for fragmented MP4 (moof detected) (D-14) |

### EXIF (`REQ-EXIF-*`)

| ID | Requirement |
|----|-------------|
| REQ-EXIF-01 | Decode EXIF IFD structures (IFD0, IFD1, ExifIFD, GPSInfoIFD, InteropIFD) |
| REQ-EXIF-02 | Handle both big-endian and little-endian byte order |
| REQ-EXIF-03 | Support all standard EXIF types (BYTE through DOUBLE) |
| REQ-EXIF-04 | ~200 tag definitions matching imagemeta's set (D-05) |
| REQ-EXIF-05 | Value converters matching exiftool behavior (APEX-to-f-number, GPS degrees-to-decimal, etc.) |
| REQ-EXIF-06 | Locate EXIF in MP4 via UUID box or meta/iloc items |
| REQ-EXIF-07 | Apple MakerNotes decoding (D-06, D-07) |
| REQ-EXIF-08 | Canon MakerNotes decoding (D-06, D-07) |
| REQ-EXIF-09 | Sony MakerNotes decoding (D-06, D-07) |

### XMP (`REQ-XMP-*`)

| ID | Requirement |
|----|-------------|
| REQ-XMP-01 | Decode XMP/RDF XML (attributes, seq/bag/alt lists) |
| REQ-XMP-02 | Handle namespace URIs |
| REQ-XMP-03 | Parse GPS coordinates from XMP format (DMS and decimal) |
| REQ-XMP-04 | Locate XMP in MP4 via UUID box (BE7ACFCB-97A9-42E8-9C71-999491E3AFAC) or meta/xml |
| REQ-XMP-05 | Main XMP packet only; skip extended XMP (D-18) |
| REQ-XMP-06 | Support HandleXMP escape hatch |

### IPTC (`REQ-IPTC-*`)

| ID | Requirement |
|----|-------------|
| REQ-IPTC-01 | Decode IPTC-IIM records (0x1C marker, record, dataset, length, value) |
| REQ-IPTC-02 | Support coded character sets (UTF-8, ISO-8859-1) |
| REQ-IPTC-03 | Handle repeatable fields as slices |
| REQ-IPTC-04 | Locate IPTC in MP4 via XMP or EXIF ApplicationNotes tag |

### QuickTime Native (`REQ-QT-*`)

| ID | Requirement |
|----|-------------|
| REQ-QT-01 | Decode moov/udta/meta/ilst iTunes-style atoms |
| REQ-QT-02 | Decode freeform `----` atoms (mean/name/data sub-atoms) |
| REQ-QT-03 | Map com.apple.quicktime.* keys (make, model, creationdate, software, location.ISO6709) |
| REQ-QT-04 | Parse mvhd: creation/modification times (1904 epoch → time.Time), duration, timescale |
| REQ-QT-05 | Parse tkhd: track dimensions, rotation matrix |
| REQ-QT-06 | Parse ISO6709 GPS coordinates to decimal degrees (D-12) |
| REQ-QT-07 | Tag names match exiftool exactly (D-09) |
| REQ-QT-08 | Preserve timezone from QuickTime creationdate (D-16) |

### CONFIG (`REQ-CFG-*`)

| ID | Requirement |
|----|-------------|
| REQ-CFG-01 | Width and height from tkhd |
| REQ-CFG-02 | Duration from mvhd (timescale-adjusted) |
| REQ-CFG-03 | Rotation from tkhd transformation matrix |
| REQ-CFG-04 | Codec fourcc and basic parameters from stsd (D-13) |

---

## 4. Non-Functional Requirements (`REQ-NF-*`)

| ID | Requirement | Decision ref |
|----|-------------|-------------|
| REQ-NF-01 | Streaming architecture; no loading entire file into memory | — |
| REQ-NF-02 | Target latency <500us for typical smartphone MP4 | — |
| REQ-NF-03 | Benchmarks included in test suite | — |
| REQ-NF-04 | All output validated against `exiftool -n -json` | D-08 |
| REQ-NF-05 | Fuzz tests for every decoder path | — |
| REQ-NF-06 | No panics on malformed input; InvalidFormatError sentinel | — |
| REQ-NF-07 | Go 1.24+ | — |
| REQ-NF-08 | Zero runtime dependencies (except golang.org/x/text) | D-01 |
| REQ-NF-09 | MIT license | — |
| REQ-NF-10 | CI: GitHub Actions with exiftool installed for golden file validation | D-08 |

---

## 5. Test Corpus Requirements (`REQ-TEST-*`)

| ID | File | Source | Priority |
|----|------|--------|----------|
| REQ-TEST-01 | iPhone H.264 MP4 with GPS | Record with GPS enabled | P0 |
| REQ-TEST-02 | iPhone H.265 HEVC MOV with GPS | Record with GPS enabled | P0 |
| REQ-TEST-03 | Minimal MP4 (mvhd only) | FFmpeg synthetic | P1 |
| REQ-TEST-04 | Truncated/corrupt MP4 | Truncate valid file | P1 |
| REQ-TEST-05 | Non-fast-start MP4 (moov at end) | FFmpeg with -movflags 0 | P1 |
| REQ-TEST-06 | Android MP4 | Record on Android | P2 |
| REQ-TEST-07 | GoPro MP4 | Source from device | P2 |
| REQ-TEST-08 | DJI drone MP4 | Source from device | P2 |
| REQ-TEST-09 | Professional camera MOV | Canon/Sony/Panasonic | P2 |
| REQ-TEST-10 | MP4 with 64-bit box sizes | Crafted or >4GB file | P2 |

---

## 6. Traceability Matrix

Mapping REQ-* → ARCH-* → source file → test file. Updated as implementation proceeds.

| Requirement | Architecture | Source File | Test File |
|-------------|-------------|-------------|-----------|
| REQ-API-01 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-02 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-03 | ARCH-IO-01, ARCH-IO-05 | io.go, videometa.go | io_test.go, videometa_test.go |
| REQ-API-04 | ARCH-BOX-05 | videometa.go, videodecoder_mp4.go | videometa_test.go |
| REQ-API-05 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-06 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-07 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-08 | ARCH-DEC-03 | metadecoder_xmp.go | videometa_test.go |
| REQ-API-09 | ARCH-ERR-05 | videometa.go | videometa_test.go |
| REQ-API-10 | ARCH-ERR-04 | videometa.go | videometa_test.go |
| REQ-API-11 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-12 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-13 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-14 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-15 | ARCH-ERR-02 | videometa.go | videometa_test.go |
| REQ-API-16 | ARCH-FLOW-01 | videometa.go | videometa_test.go |
| REQ-API-17 | ARCH-ERR-05 | videometa.go | videometa_test.go |
| REQ-API-18 | ARCH-ERR-05 | videometa.go, videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-01 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-02 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-03 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-04 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-05 | ARCH-BOX-03 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-06 | ARCH-BOX-05 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-07 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-BOX-08 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-EXIF-01 | ARCH-DEC-02 | metadecoder_exif.go | metadecoder_exif_test.go |
| REQ-EXIF-02 | ARCH-DEC-02, ARCH-IO-02 | metadecoder_exif.go, io.go | metadecoder_exif_test.go |
| REQ-EXIF-03 | ARCH-DEC-02 | metadecoder_exif.go | metadecoder_exif_test.go |
| REQ-EXIF-04 | ARCH-DEC-02 | metadecoder_exif_fields.go | videometa_test.go |
| REQ-EXIF-05 | ARCH-DEC-06 | metadecoder_exif.go | metadecoder_exif_test.go |
| REQ-EXIF-06 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-EXIF-07 | ARCH-DEC-08 | metadecoder_makernotes_pentax.go | videometa_test.go |
| REQ-EXIF-08 | ARCH-DEC-08 | metadecoder_makernotes_pentax.go | videometa_test.go |
| REQ-EXIF-09 | ARCH-DEC-08 | metadecoder_makernotes_pentax.go | videometa_test.go |
| REQ-XMP-01 | ARCH-DEC-03 | metadecoder_xmp.go | metadecoder_xmp_test.go |
| REQ-XMP-02 | ARCH-DEC-03 | metadecoder_xmp.go | metadecoder_xmp_test.go |
| REQ-XMP-03 | ARCH-DEC-03 | metadecoder_xmp.go | metadecoder_xmp_test.go |
| REQ-XMP-04 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-XMP-05 | ARCH-DEC-03 | metadecoder_xmp.go | metadecoder_xmp_test.go |
| REQ-XMP-06 | ARCH-DEC-03 | metadecoder_xmp.go | metadecoder_xmp_test.go |
| REQ-IPTC-01 | ARCH-DEC-04 | metadecoder_iptc.go | metadecoder_iptc_test.go |
| REQ-IPTC-02 | ARCH-DEC-04 | metadecoder_iptc.go | metadecoder_iptc_test.go |
| REQ-IPTC-03 | ARCH-DEC-04 | metadecoder_iptc.go | metadecoder_iptc_test.go |
| REQ-IPTC-04 | ARCH-DEC-04 | metadecoder_exif.go, metadecoder_iptc.go | metadecoder_iptc_test.go |
| REQ-QT-01 | ARCH-DEC-05 | metadecoder_quicktime.go | videometa_test.go |
| REQ-QT-02 | ARCH-DEC-05 | metadecoder_quicktime.go | videometa_test.go |
| REQ-QT-03 | ARCH-DEC-05 | metadecoder_quicktime.go | videometa_test.go |
| REQ-QT-04 | ARCH-DEC-05, ARCH-BOX-04 | metadecoder_quicktime.go, videodecoder_mp4.go | videometa_test.go |
| REQ-QT-05 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-QT-06 | ARCH-DEC-05 | helpers.go | helpers_test.go, videometa_test.go |
| REQ-QT-07 | ARCH-DEC-07 | metadecoder_quicktime_fields.go | videometa_test.go |
| REQ-QT-08 | ARCH-DEC-05 | metadecoder_quicktime.go | videometa_test.go |
| REQ-CFG-01 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-CFG-02 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-CFG-03 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-CFG-04 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go |
| REQ-NF-01 | ARCH-IO-01 | io.go | io_test.go |
| REQ-NF-02 | ARCH-IO-04 | io.go | videometa_bench_test.go |
| REQ-NF-03 | ARCH-TEST-06 | videometa_bench_test.go | — |
| REQ-NF-04 | ARCH-TEST-01, ARCH-TEST-02 | gen/main.go | videometa_test.go |
| REQ-NF-05 | ARCH-TEST-05 | videometa_fuzz_test.go | — |
| REQ-NF-06 | ARCH-ERR-01 | helpers.go | videometa_test.go, videometa_fuzz_test.go |
| REQ-NF-07 | — | go.mod | — |
| REQ-NF-08 | ARCH-DEP-01 | go.mod | — |
| REQ-NF-09 | — | LICENSE | — |
| REQ-NF-10 | ARCH-TEST-03 | .github/workflows/ci.yml | — |
| REQ-TEST-01 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-02 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-03 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-04 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-05 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-06 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-07 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-08 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-09 | ARCH-TEST-01 | testdata/ | videometa_test.go |
| REQ-TEST-10 | ARCH-TEST-01 | testdata/ | videometa_test.go |
