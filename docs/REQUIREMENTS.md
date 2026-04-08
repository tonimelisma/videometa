# videometa Requirements

Module: `github.com/tonimelisma/videometa`

Every requirement has a unique ID (`REQ-*`) for traceability to architecture (`ARCH-*`), source files, and tests.

## Decisions Registry

| ID | Decision | Choice |
|----|----------|--------|
| D-01 | Runtime dependencies | Zero runtime dependencies |
| D-02 | Tag overlap handling | Preserve all tags by source + namespace + tag + occurrence order; public collection views remain lossless |
| D-03 | VideoFormat enum | Single MP4 value; auto-detect MOV vs MP4 internally from ftyp brand |
| D-04 | Format detection | Optional; auto-detect from ftyp if not specified |
| D-08 | Test system | Committed grouped exiftool JSON + duplicate-preserving ordered exiftool goldens + CI validation via GitHub Actions |
| D-09 | Tag names | Match exiftool exactly |
| D-10 | Large file handling | Prefer `io.ReadSeeker`; `io.Reader` fallback with degraded performance |
| D-11 | Unknown boxes | Skip silently; warn only for malformed supported metadata payloads |
| D-12 | GPS coordinates | Decimal degrees float64 matching exiftool `-n` |
| D-13 | Codec info | Extract codec fourcc + basic params via `CONFIG` |
| D-14 | Fragmented MP4 | Not supported in v1; return error if detected |
| D-15 | Thumbnails/cover art | No generated thumbnails or preview extraction; embedded metadata cover art stays in scope |
| D-16 | Timestamp type | Go `time.Time`; preserve original timezone; `GetDateTimeUTC()` convenience |
| D-17 | Partial reads | Best-effort mode; succeed if `moov` can still be reached, otherwise return partial + error |
| D-19 | Convenience API | `DecodeAll()` returns `Metadata{Tags, VideoConfig}` |
| D-23 | Source taxonomy | `QUICKTIME` for standard/container-native routes, `VENDOR` for vendor-specific container metadata families |
| D-24 | Validation policy | `Validated` requires real media fixtures only; synthetic tests are regression coverage only |
| D-25 | Namespace contract | `Namespace` strings are stable route identities; QuickTime namespaces include 1-based track ordinals and vendor namespaces use `VendorName/route` |
| D-26 | Namespace subviews | `SourceTags.Namespace(name)` returns a lossless `NamespaceTags` view; repeated tags stay queryable via `Find` |
| D-27 | Embedded image metadata | Explicit non-goal; EXIF/TIFF, XMP/RDF, and IPTC-IIM payloads are not parsed even if carried inside video containers |
| D-28 | Release cadence | Every merge to `main` is release-producing |
| D-29 | Version source of truth | `VERSION` file |
| D-30 | Release gate | Publish only after hosted CI restores the bootstrap-downloadable validated fixtures and passes on the exact `main` commit |
| D-31 | Fixture enforcement | Bootstrap-downloadable real-fixture tests may skip by default in ordinary local development, but must fail hard when `VIDEOMETA_REQUIRE_LOCAL_FIXTURES=1` |

---

## 1. Scope Statement

- Read-only metadata extraction from MP4/MOV (ISOBMFF containers)
- Pure Go, no CGo, no external binaries
- v1: MP4/MOV only
- No writing, no transcoding, no generated thumbnails/preview extraction, no fragmented MP4
- Explicit non-goal: embedded image metadata payloads such as EXIF/TIFF IFDs, XMP/RDF packets, and IPTC-IIM records, including container routes like `uuid`, `XMP_`, `meta/iloc`, and EXIF `ApplicationNotes`

---

## 2. API Requirements (`REQ-API-*`)

| ID | Requirement | Decision ref |
|----|-------------|-------------|
| REQ-API-01 | `Decode(Options) (DecodeResult, error)` entry point | — |
| REQ-API-02 | `DecodeAll(Options) (Metadata, error)` convenience wrapper | D-19 |
| REQ-API-03 | `Options.R` accepts `io.ReadSeeker`; `io.Reader` fallback with degraded performance | D-10 |
| REQ-API-04 | `Options.VideoFormat` optional; auto-detect from `ftyp` if omitted | D-03, D-04 |
| REQ-API-05 | `Source` bitmask: `QUICKTIME \| VENDOR \| CONFIG \| COMPOSITE` | D-23 |
| REQ-API-06 | `HandleTag` callback receives `TagInfo{Source, Tag, Namespace, Value}` | — |
| REQ-API-07 | `ShouldHandleTag`, `LimitNumTags`, and `LimitTagSize` control emission and collection | — |
| REQ-API-09 | `Warnf` callback for non-fatal warnings on supported metadata paths | — |
| REQ-API-10 | `Timeout` for decode operations | — |
| REQ-API-11 | `Tags.GetDateTime()` returns the best available creation time with original timezone preserved | D-16 |
| REQ-API-12 | `Tags.GetDateTimeUTC()` normalizes to UTC | D-16 |
| REQ-API-13 | `Tags.GetLatLong()` returns `(lat, lon float64)` in decimal degrees from supported video metadata | D-12 |
| REQ-API-14 | `DecodeResult.VideoConfig` provides Width, Height, Duration, Rotation, Codec | D-13 |
| REQ-API-15 | `ErrStopWalking` sentinel for early termination | — |
| REQ-API-16 | Collected tags preserve source + namespace + tag + decode order; no lossy flattening in the public API | D-02 |
| REQ-API-17 | Empty result (no error) when a file has no supported metadata | D-17 |
| REQ-API-18 | Best-effort partial mode for files where `moov` position is unknown | D-17 |
| REQ-API-19 | `Tags.All()` returns ordered `[]TagInfo`; per-source access is namespace-aware through lossless `SourceTags` / `NamespaceTags` views | D-02, D-19, D-26 |
| REQ-API-20 | `CONFIG` is a request flag for `DecodeResult.VideoConfig`, not a tag group | D-13, D-19 |
| REQ-API-21 | `COMPOSITE` is output-only from `DecodeAll`; `Decode` never emits composite tags | D-19 |
| REQ-API-22 | `SourceTags.FindInNamespace()` and `NamespaceTags.Find()` preserve repeated tags within one namespace in decode order | D-02, D-25, D-26 |

---

## 3. Metadata Source Requirements

### ISOBMFF Navigation (`REQ-BOX-*`)

| ID | Requirement |
|----|-------------|
| REQ-BOX-01 | Parse standard box headers (4-byte size + 4-byte fourcc) |
| REQ-BOX-02 | Support 64-bit extended box sizes (`size=1`) |
| REQ-BOX-03 | Support boxes extending to EOF (`size=0`) |
| REQ-BOX-04 | Parse FullBox headers for supported containers (`meta`, `mvhd`, `tkhd`, etc.) |
| REQ-BOX-05 | Handle `moov` at end of file by seeking or discarding past `mdat` |
| REQ-BOX-06 | Validate `ftyp` brand for MP4/MOV compatibility |
| REQ-BOX-07 | Skip unknown boxes silently; warn only for malformed supported metadata payloads |
| REQ-BOX-08 | Return error for fragmented MP4 (`moof` detected) |

### QuickTime / Container Metadata (`REQ-QT-*`)

| ID | Requirement |
|----|-------------|
| REQ-QT-01 | Decode `moov/udta/meta/ilst` iTunes-style atoms |
| REQ-QT-02 | Decode freeform `----` atoms (`mean`/`name`/`data`) |
| REQ-QT-03 | Decode validated `mdta`/freeform/container-native tag mappings exactly as exiftool names them |
| REQ-QT-04 | Parse `mvhd` timestamps/duration and other validated structural QuickTime tags exactly as exiftool emits them |
| REQ-QT-05 | Parse `tkhd` dimensions and rotation matrix |
| REQ-QT-06 | Parse ISO6709 GPS coordinates to decimal degrees |
| REQ-QT-07 | Tag names and value formatting match exiftool exactly |
| REQ-QT-08 | Preserve timezone from QuickTime `CreationDate` |

### Vendor Container Metadata (`REQ-VENDOR-*`)

| ID | Requirement |
|----|-------------|
| REQ-VENDOR-01 | Decode validated vendor container metadata families: Pentax `moov/udta/TAGS`, Sony UUID-PROF, Sony UUID-USMT/MTDT, Sony NRTM, and GoPro `udta`/GPMF |
| REQ-VENDOR-02 | Vendor tags emit as `Source=VENDOR` with namespaces of the form `VendorName/route` |
| REQ-VENDOR-03 | Repeated vendor tags remain lossless and queryable in decode order |
| REQ-VENDOR-04 | Validated vendor real fixtures match exiftool exactly for emitted tag names, values, and repeated-tag occurrence order |

### CONFIG (`REQ-CFG-*`)

| ID | Requirement |
|----|-------------|
| REQ-CFG-01 | Width and height from validated visual track metadata |
| REQ-CFG-02 | Duration from `mvhd` (timescale-adjusted) |
| REQ-CFG-03 | Rotation from `tkhd` transformation matrix |
| REQ-CFG-04 | Codec fourcc and validated sample-description parameters from `stsd` |

---

## 4. Non-Functional Requirements (`REQ-NF-*`)

| ID | Requirement | Decision ref |
|----|-------------|-------------|
| REQ-NF-01 | Streaming architecture; no loading entire files into memory | — |
| REQ-NF-02 | Target latency <500us for typical smartphone MP4 | — |
| REQ-NF-03 | Benchmarks included in the test suite | — |
| REQ-NF-04 | Real-file golden validation uses both grouped `exiftool -n -json -g` output and duplicate-preserving ordered `exiftool -a -n -G0 -S` output | D-08 |
| REQ-NF-05 | Fuzz tests for every supported decoder path | — |
| REQ-NF-06 | No panics on malformed input; `InvalidFormatError` sentinel | — |
| REQ-NF-07 | Go 1.25+ | — |
| REQ-NF-08 | Zero runtime dependencies | D-01 |
| REQ-NF-09 | MIT license | — |
| REQ-NF-10 | Hosted CI runs format, lint, build, tests, coverage, release-guard checks, and exiftool-backed golden validation | D-08, D-28, D-29 |
| REQ-NF-11 | Hosted CI restores the bootstrap-downloadable validated fixture corpus and fails hard when required bootstrap fixtures are missing in release mode | D-24, D-30, D-31 |
| REQ-NF-12 | Pushes to `main` publish an annotated semver tag and GitHub Release from `VERSION` and `docs/releases/<VERSION>.md` only after hosted verification passes | D-28, D-29, D-30 |
| REQ-NF-13 | `main` branch protection requires pull-request-based merges, up-to-date required checks, and conversation resolution before merge | D-28, D-30 |

---

## 5. Test Corpus Requirements (`REQ-TEST-*`)

| ID | File | Source | Priority |
|----|------|--------|----------|
| REQ-TEST-01 | iPhone H.264 MP4 with GPS | committed real fixture (`testdata/with_gps.mp4`) | P0 |
| REQ-TEST-02 | iPhone HEVC MOV | committed real fixture (`testdata/IMG_5179.MOV`) | P0 |
| REQ-TEST-03 | Minimal MP4 | committed synthetic fixture (`testdata/minimal.mp4`) | P1 |
| REQ-TEST-04 | Malformed/corrupt MP4 regression inputs | crafted inline malformed byte sequences | P1 |
| REQ-TEST-05 | Non-fast-start MP4 (`moov` at end) | committed synthetic fixture (`testdata/nonfaststart.mp4`) | P1 |
| REQ-TEST-06 | Android MP4 | committed real fixture (`testdata/google.mp4`) | P2 |
| REQ-TEST-07 | GoPro MP4 | bootstrap-downloadable public GoPro HERO12 clip (`testdata/gopro_action.mp4`) | P2 |
| REQ-TEST-08 | DJI drone/pro camera MOV | bootstrap-downloadable public DJI Inspire 3 clip (`testdata/dji_inspire3_car_4k120_rec709.mov`) | P2 |
| REQ-TEST-09 | Professional camera MOV/MP4 | committed Sony A6700 clip plus bootstrap-downloadable DJI Ronin 4D clip | P2 |
| REQ-TEST-10 | MP4 with 64-bit box sizes | crafted or >4GB fixture | P2 |

---

## 6. Traceability Matrix

Status terms used below:

- `Validated`: fully backed by the cited tests using real media fixtures only
- `Implemented`: code exists and has unit/fuzz/regression coverage, but no qualifying real-file evidence exists yet
- `Static`, `Config`, and `Pending` keep their usual meanings

| Requirement | Architecture | Source File | Test File | Status |
|-------------|-------------|-------------|-----------|--------|
| REQ-API-01 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-02 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-03 | ARCH-IO-01, ARCH-IO-03 | io.go, videometa.go | io_test.go, videometa_test.go | Validated |
| REQ-API-04 | ARCH-BOX-05 | videometa.go, videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-API-05 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-06 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-07 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-09 | ARCH-ERR-05 | videometa.go, metadecoder_sony_nrtm.go | videometa_test.go | Implemented |
| REQ-API-10 | ARCH-ERR-04 | videometa.go | videometa_test.go | Validated |
| REQ-API-11 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-12 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-13 | ARCH-FLOW-01 | videometa.go, gps.go, value.go | videometa_test.go, gps_test.go, videometa_golden_test.go | Validated |
| REQ-API-14 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-15 | ARCH-ERR-02 | videometa.go | videometa_test.go | Validated |
| REQ-API-16 | ARCH-DEC-03 | videometa.go | videometa_test.go | Validated |
| REQ-API-17 | ARCH-ERR-05 | videometa.go | videometa_test.go | Implemented |
| REQ-API-18 | ARCH-ERR-05 | videometa.go, videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-API-19 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-20 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-21 | ARCH-FLOW-01 | videometa.go | videometa_test.go | Validated |
| REQ-API-22 | ARCH-DEC-03 | videometa.go | videometa_test.go | Validated |
| REQ-BOX-01 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-BOX-02 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go | Implemented |
| REQ-BOX-03 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go | Implemented |
| REQ-BOX-04 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-BOX-05 | ARCH-BOX-03 | videodecoder_mp4.go | videometa_test.go, videometa_golden_test.go | Validated |
| REQ-BOX-06 | ARCH-BOX-05 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-BOX-07 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go | Implemented |
| REQ-BOX-08 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go | Implemented |
| REQ-QT-01 | ARCH-DEC-01 | metadecoder_quicktime.go | metadecoder_quicktime_test.go, videometa_golden_test.go | Validated |
| REQ-QT-02 | ARCH-DEC-01 | metadecoder_quicktime.go | metadecoder_quicktime_test.go, videometa_golden_test.go | Validated |
| REQ-QT-03 | ARCH-DEC-01 | metadecoder_quicktime.go | metadecoder_quicktime_test.go, videometa_golden_test.go | Validated |
| REQ-QT-04 | ARCH-DEC-01, ARCH-BOX-04 | metadecoder_quicktime.go, videodecoder_mp4.go | videometa_golden_test.go, videometa_fixture_test.go | Validated |
| REQ-QT-05 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-QT-06 | ARCH-DEC-04 | gps.go | gps_test.go, videometa_golden_test.go | Validated |
| REQ-QT-07 | ARCH-DEC-01 | metadecoder_quicktime.go | videometa_golden_test.go | Validated |
| REQ-QT-08 | ARCH-DEC-01 | metadecoder_quicktime.go | videometa_test.go | Validated |
| REQ-VENDOR-01 | ARCH-DEC-02 | metadecoder_quicktime_pentax.go, metadecoder_sony_nrtm.go, metadecoder_quicktime_gopro.go, videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-VENDOR-02 | ARCH-DEC-02 | videometa.go, videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-VENDOR-03 | ARCH-DEC-03 | videometa.go | videometa_test.go | Implemented |
| REQ-VENDOR-04 | ARCH-TEST-02 | metadecoder_quicktime_pentax.go, metadecoder_sony_nrtm.go, metadecoder_quicktime_gopro.go, videodecoder_mp4.go | videometa_golden_test.go | Validated |
| REQ-CFG-01 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-CFG-02 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go | Validated |
| REQ-CFG-03 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go, quicktime_matrix_test.go | Validated |
| REQ-CFG-04 | ARCH-BOX-04 | videodecoder_mp4.go | videometa_test.go, videometa_fixture_test.go | Validated |
| REQ-NF-01 | ARCH-IO-01 | io.go | io_test.go, videometa_alloc_test.go, videometa_sony_nrtm_test.go | Implemented |
| REQ-NF-02 | ARCH-IO-04 | io.go | videometa_latency_test.go, videometa_bench_test.go | Validated |
| REQ-NF-03 | ARCH-TEST-03 | videometa_bench_test.go | videometa_bench_test.go | Static |
| REQ-NF-04 | ARCH-TEST-01 | gen/main.go | videometa_golden_test.go | Validated |
| REQ-NF-05 | ARCH-TEST-04 | videometa_fuzz_test.go | videometa_fuzz_test.go | Implemented |
| REQ-NF-06 | ARCH-ERR-01 | errors.go | videometa_test.go, errors_test.go | Implemented |
| REQ-NF-07 | — | go.mod | — | Static |
| REQ-NF-08 | ARCH-DEP-01 | go.mod | — | Static |
| REQ-NF-09 | — | LICENSE | — | Static |
| REQ-NF-10 | ARCH-TEST-05 | .github/workflows/ci.yml, scripts/check-format.sh, scripts/check-release-guard.sh | — | Config |
| REQ-NF-11 | ARCH-TEST-06 | .github/workflows/ci.yml, scripts/check-local-fixtures.sh, testhelpers_test.go | — | Config |
| REQ-NF-12 | ARCH-REL-01 | VERSION, docs/releases/, .github/workflows/release.yml | — | Config |
| REQ-NF-13 | ARCH-REL-02 | scripts/configure-branch-protection.sh | — | Config |
| REQ-TEST-01 | ARCH-TEST-01 | testdata/with_gps.mp4 | videometa_golden_test.go | Validated |
| REQ-TEST-02 | ARCH-TEST-01 | testdata/IMG_5179.MOV | videometa_golden_test.go | Validated |
| REQ-TEST-03 | ARCH-TEST-01 | testdata/minimal.mp4 | videometa_golden_test.go | Validated |
| REQ-TEST-04 | ARCH-TEST-04 | inline malformed byte slices | videometa_test.go | Implemented |
| REQ-TEST-05 | ARCH-TEST-01 | testdata/nonfaststart.mp4 | videometa_golden_test.go | Validated |
| REQ-TEST-06 | ARCH-TEST-01 | testdata/google.mp4 | videometa_fixture_test.go, videometa_golden_test.go | Validated |
| REQ-TEST-07 | ARCH-TEST-01 | testdata/gopro_action.mp4 | videometa_fixture_test.go, videometa_golden_test.go | Validated |
| REQ-TEST-08 | ARCH-TEST-01 | testdata/dji_inspire3_car_4k120_rec709.mov | videometa_fixture_test.go, videometa_golden_test.go | Validated |
| REQ-TEST-09 | ARCH-TEST-01 | testdata/sony_a6700.mp4, testdata/dji_ronin4d_4k_prores4444_25fps.mov | videometa_fixture_test.go, videometa_golden_test.go | Validated |
| REQ-TEST-10 | ARCH-BOX-02 | videodecoder_mp4.go | videometa_test.go | Implemented |
