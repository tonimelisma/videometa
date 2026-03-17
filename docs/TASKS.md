# videometa Tasks

Tasks organized by milestone. Each task traces to requirements (`REQ-*`) and architecture (`ARCH-*`).

---

## Milestone 1: Foundation

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M1-01 | `go mod init github.com/tonimelisma/videometa` | — | go.mod | Module initializes |
| TASK-M1-02 | Implement streamReader | ARCH-IO-* | io.go, io_test.go | Unit tests pass for read1/2/4/8, seek, byte order |
| TASK-M1-03 | Implement helpers | — | helpers.go, helpers_test.go | Rat[T], InvalidFormatError, value converters work |
| TASK-M1-04 | Define public API types (stubs) | REQ-API-* | videometa.go | Types compile, Decode returns "not implemented" |
| TASK-M1-05 | Acquire P0 test videos | REQ-TEST-01, 02 | testdata/ | iPhone MP4 + MOV with GPS present |
| TASK-M1-06 | Build golden file generator | ARCH-TEST-01 | gen/main.go | `go generate ./gen` produces JSON from exiftool |
| TASK-M1-07 | Set up GitHub Actions CI | REQ-NF-10 | .github/workflows/ci.yml | CI runs tests + exiftool validation |

---

## Milestone 2: ISOBMFF + CONFIG

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M2-01 | Implement box header parser | REQ-BOX-01..04 | videodecoder_mp4.go | Parses 32/64-bit boxes, FullBox, size=0 |
| TASK-M2-02 | Implement box traversal + routing | ARCH-BOX-01, 04 | videodecoder_mp4.go | Iterates box tree, dispatches to handlers |
| TASK-M2-03 | Implement ftyp validation | REQ-BOX-06, ARCH-BOX-05 | videodecoder_mp4.go | Detects MP4 vs MOV; rejects unknown brands |
| TASK-M2-04 | Implement mvhd parser | REQ-QT-04, REQ-CFG-02 | videodecoder_mp4.go | Creation time, duration, timescale correct |
| TASK-M2-05 | Implement tkhd parser | REQ-QT-05, REQ-CFG-01, 03 | videodecoder_mp4.go | Width, height, rotation correct |
| TASK-M2-06 | Implement stsd codec extraction | REQ-CFG-04 | videodecoder_mp4.go | Codec fourcc + params extracted |
| TASK-M2-07 | Wire up Decode() | REQ-API-01 | videometa.go | End-to-end: decode iPhone MP4, get VideoConfig |
| TASK-M2-08 | Implement io.Reader fallback | ARCH-IO-05, D-10 | io.go | Decode works with io.Reader (slower) |
| TASK-M2-09 | Golden file test for CONFIG | ARCH-TEST-02 | videometa_test.go | CONFIG output matches exiftool |

---

## Milestone 3: QuickTime Native Metadata

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M3-01 | Implement ilst atom parser | REQ-QT-01 | metadecoder_quicktime.go | Standard keys (©nam, ©ART, ©day, etc.) decoded |
| TASK-M3-02 | Build QuickTime tag name table | REQ-QT-07 | metadecoder_quicktime_fields.go | Names match exiftool output |
| TASK-M3-03 | Implement freeform atom parser | REQ-QT-02, 03 | metadecoder_quicktime.go | com.apple.quicktime.* keys decoded |
| TASK-M3-04 | Implement ISO6709 GPS parser | REQ-QT-06 | helpers.go | Parses "+34.0592-118.4460+042.938/" format |
| TASK-M3-05 | Wire up QT decoder to box router | ARCH-BOX-04 | videodecoder_mp4.go | ilst/freeform atoms reach QT decoder |
| TASK-M3-06 | Golden file test for QUICKTIME | ARCH-TEST-02 | videometa_test.go | QT output matches exiftool |

---

## Milestone 4: XMP Decoder

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M4-01 | Implement XMP/RDF XML parser | REQ-XMP-01..03 | metadecoder_xmp.go | Attributes, seq/bag/alt, GPS parsed |
| TASK-M4-02 | Locate XMP in MP4 (UUID box) | REQ-XMP-04 | videodecoder_mp4.go | UUID box detected and routed |
| TASK-M4-03 | Implement HandleXMP escape hatch | REQ-XMP-06 | metadecoder_xmp.go | Custom handler receives raw XMP reader |
| TASK-M4-04 | Golden file test for XMP | ARCH-TEST-02 | videometa_test.go | XMP output matches exiftool |

---

## Milestone 5: EXIF Decoder

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M5-01 | Build EXIF tag name table | REQ-EXIF-04 | metadecoder_exif_fields.go | ~200 tags defined |
| TASK-M5-02 | Implement EXIF IFD parser | REQ-EXIF-01..03 | metadecoder_exif.go | IFD traversal, type reads, pointer following |
| TASK-M5-03 | Implement EXIF value converters | REQ-EXIF-05 | metadecoder_exif.go | APEX→f-number, GPS deg→decimal, etc. |
| TASK-M5-04 | Locate EXIF in MP4 | REQ-EXIF-06 | videodecoder_mp4.go | UUID/iloc EXIF detected and routed |
| TASK-M5-05 | GPS tag handling | REQ-EXIF-05 | metadecoder_exif.go | Lat/Lon/Alt with ref tags, degree conversion |
| TASK-M5-06 | Golden file test for EXIF | ARCH-TEST-02 | videometa_test.go | EXIF output matches exiftool |

---

## Milestone 6: IPTC Decoder

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M6-01 | Implement IPTC record parser | REQ-IPTC-01..03 | metadecoder_iptc.go | Records decoded, charset handled |
| TASK-M6-02 | Build IPTC field definitions | REQ-IPTC-01 | metadecoder_iptc_fields.json | JSON embedded, parsed at init |
| TASK-M6-03 | Locate IPTC in MP4 | REQ-IPTC-04 | videodecoder_mp4.go | Found via XMP or EXIF ApplicationNotes |
| TASK-M6-04 | Golden file test for IPTC | ARCH-TEST-02 | videometa_test.go | IPTC output matches exiftool |

---

## Milestone 7: Convenience + Polish

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M7-01 | Implement DecodeAll() | REQ-API-02 | videometa.go | Returns populated Tags struct |
| TASK-M7-02 | Implement Tags.GetDateTime() | REQ-API-11 | videometa.go | Returns time.Time with priority: EXIF→XMP→QT→IPTC |
| TASK-M7-03 | Implement Tags.GetDateTimeUTC() | REQ-API-12 | videometa.go | Returns UTC-normalized time.Time |
| TASK-M7-04 | Implement Tags.GetLatLong() | REQ-API-13 | videometa.go | Returns (lat, lon) with priority: EXIF→XMP→QT |
| TASK-M7-05 | Implement ShouldHandleTag/Limits | REQ-API-07 | videometa.go | Filtering and limits work |
| TASK-M7-06 | Implement timeout mechanism | REQ-API-10, ARCH-ERR-04 | videometa.go | Decode respects timeout |
| TASK-M7-07 | Implement auto-detect from ftyp | REQ-API-04 | videometa.go | Format optional, detected automatically |

---

## Milestone 8: Robustness + Testing

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M8-01 | Fuzz test: FuzzDecodeMP4 | REQ-NF-05 | videometa_fuzz_test.go | No panics on random input |
| TASK-M8-02 | Acquire P1 test files | REQ-TEST-03..05 | testdata/ | Minimal, corrupt, moov-at-end files |
| TASK-M8-03 | Test: corrupt/truncated MP4 | REQ-NF-06 | videometa_test.go | InvalidFormatError returned |
| TASK-M8-04 | Test: 64-bit box sizes | REQ-BOX-02 | videometa_test.go | Large boxes handled correctly |
| TASK-M8-05 | Test: moov-at-end | REQ-BOX-05 | videometa_test.go | Metadata extracted from non-fast-start file |
| TASK-M8-06 | Test: fragmented MP4 rejection | REQ-BOX-08 | videometa_test.go | Error returned for fMP4 |
| TASK-M8-07 | Test: no-metadata file | REQ-API-17 | videometa_test.go | Empty result, no error |
| TASK-M8-08 | Test: io.Reader fallback | ARCH-IO-05 | videometa_test.go | Decode works without seeking |
| TASK-M8-09 | Benchmark suite | REQ-NF-02, 03 | videometa_bench_test.go | Per-source + all-sources benchmarks |

---

## Milestone 9: MakerNotes

| Task | Description | Traces to | Files | Acceptance |
|------|-------------|-----------|-------|------------|
| TASK-M9-01 | Apple MakerNotes tag table + decoder | REQ-EXIF-07 | metadecoder_makernotes_apple.go | Apple MakerNotes match exiftool |
| TASK-M9-02 | Canon MakerNotes tag table + decoder | REQ-EXIF-08 | metadecoder_makernotes_canon.go | Canon MakerNotes match exiftool |
| TASK-M9-03 | Sony MakerNotes tag table + decoder | REQ-EXIF-09 | metadecoder_makernotes_sony.go | Sony MakerNotes match exiftool |
| TASK-M9-04 | Golden file tests for MakerNotes | ARCH-TEST-02 | videometa_test.go | MakerNotes output matches exiftool |

---

## Milestone 10: Documentation + Release

| Task | Description | Files | Acceptance |
|------|-------------|-------|------------|
| TASK-M10-01 | Write README.md | README.md | Usage examples, benchmarks, exiftool comparison |
| TASK-M10-02 | Update CLAUDE.md for maintenance phase | CLAUDE.md | Reflects shipped state |
| TASK-M10-03 | go vet + staticcheck + golangci-lint | — | All pass clean |
| TASK-M10-04 | Tag v0.1.0 | — | Release tagged |

---

## Milestone 11: Extended Test Coverage (post-v0.1.0)

| Task | Description | Traces to | Files |
|------|-------------|-----------|-------|
| TASK-M11-01 | Android MP4 test file + golden test | REQ-TEST-06 | testdata/ |
| TASK-M11-02 | GoPro MP4 test file + golden test | REQ-TEST-07 | testdata/ |
| TASK-M11-03 | DJI drone MP4 test file + golden test | REQ-TEST-08 | testdata/ |
| TASK-M11-04 | Pro camera MOV test file + golden test | REQ-TEST-09 | testdata/ |
| TASK-M11-05 | 64-bit box size test file | REQ-TEST-10 | testdata/ |

---

## Test Video Acquisition (parallel with Milestones 1-2)

| Priority | What | How | Verify with |
|----------|------|-----|-------------|
| P0 | iPhone H.264 MP4 | Record 5-10s video with GPS enabled | `exiftool -v3 -g file.mp4` |
| P0 | iPhone HEVC MOV | Record with HEVC enabled + GPS | Same |
| P1 | Minimal MP4 | `ffmpeg -f lavfi -i color=black:s=320x240:d=1 -an minimal.mp4` | `exiftool -v3` |
| P1 | Truncated MP4 | `dd if=valid.mp4 of=truncated.mp4 bs=1 count=1000` | Should trigger InvalidFormatError |
| P1 | Non-fast-start | `ffmpeg -f lavfi -i color=black:s=320x240:d=5 -movflags 0 nonfaststart.mp4` | moov at end verified with `exiftool -v3` |
| P2 | Android/GoPro/DJI/Pro camera | Source from devices or colleagues | `exiftool -v3 -g` to document metadata |
