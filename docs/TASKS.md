# videometa Tasks

Tasks organized by milestone. Each task traces to requirements (`REQ-*`) and architecture (`ARCH-*`).

## Status Summary

| Milestone | Status |
|-----------|--------|
| M1: Foundation | ✅ Complete |
| M2: ISOBMFF + CONFIG | ✅ Complete |
| M3: QuickTime Native | ✅ Complete |
| M4: XMP Decoder | ✅ Complete |
| M5: EXIF Decoder | ✅ Complete |
| M6: IPTC Decoder | ✅ Complete |
| M7: Convenience + Polish | ✅ Complete |
| M8: Robustness + Testing | ✅ Complete |
| M9: Manufacturer Metadata | ✅ Complete |
| M10: Documentation + Release | ✅ Complete |
| M11: Extended Test Coverage | ✅ Complete |
| M12: Implement Skipped Tests | ✅ Complete |
| M13: Fix Weak Tests | ✅ Complete |
| M14: Test & Error Robustness | ✅ Complete |
| M15: Test & IO Cleanup | ✅ Complete |
| M16: Integrity Recovery | ✅ Complete |
| M17: Oracle Evidence Hardening | ✅ Complete |
| M18: NRTM Streaming Guard | ✅ Complete |
| M20: Pre-1.0 API Reset | ✅ Complete |
| M21: API Hardening + Evidence Policy | ✅ Complete |
| M22: Fixture Acquisition Bootstrap | ✅ Complete |

---

## Milestone 1–8: Core Implementation

All complete. See git history for details.

---

## Milestone 9: Manufacturer-Specific Video Metadata

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M9-01 | Pentax TAGS binary parser (7 tags: Make, ExposureTime, FNumber, ExposureCompensation, WhiteBalance, FocalLength, ISO) | ✅ Complete | metadecoder_quicktime_pentax.go |
| TASK-M9-02 | Sony XAVC UUID-PROF (19 tags: file/video/audio profiles) | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-03 | Sony UUID-USMT/MTDT (TrackProperty, TimeZone) | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-04 | Sony NonRealTimeMeta XML parser (35 tags) | ✅ Complete | metadecoder_sony_nrtm.go |
| TASK-M9-05 | Apple MOV mdta locale handling (-eng-US suffixes) | ✅ Complete | metadecoder_quicktime.go |
| TASK-M9-06 | Apple wave/frma (PurchaseFileFormat) | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-07 | Apple gmhd/gmin (GenMediaVersion, GenFlags, etc.) | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-08 | tref/cdsc (ContentDescribes) + MetaFormat from stsd | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-09 | Old-style QuickTime text atoms (©fmt, ©inf) in udta | ✅ Complete | videodecoder_mp4.go |
| TASK-M9-10 | XMP from XMP_ box in udta + XMPToolkit extraction | ✅ Complete | videodecoder_mp4.go, metadecoder_xmp.go |
| TASK-M9-11 | Golden file tests for all manufacturer tags | ✅ Complete | videometa_golden_test.go |

---

## Milestone 10: Documentation + Release

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M10-01 | README with usage examples and benchmarks | ✅ Complete | README.md |
| TASK-M10-02 | Update CLAUDE.md for v0.1.0 | ✅ Complete | CLAUDE.md |
| TASK-M10-03 | golangci-lint clean | ✅ Complete | — |
| TASK-M10-04 | Tag v0.1.0 | ✅ Complete | — |

---

## Milestone 11: Exhaustive Tests + Composite Tags

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M11-01 | Exhaustive golden tests for all 6 test files (100% tag coverage) | ✅ Complete | videometa_golden_test.go |
| TASK-M11-02 | Composite tag emission (ImageSize, Megapixels, AvgBitrate, Rotation, GPS*, Aperture, ShutterSpeed, FocalLength35efl, LightValue, LensID) | ✅ Complete | videometa.go |
| TASK-M11-03 | Fix malformed-input, Warnf callback, and timeout tests | ✅ Complete | videometa_test.go |
| TASK-M11-04 | Add traceability comments (99 total // Validates: entries) | ✅ Complete | *_test.go |
| TASK-M11-05 | Fix decoder bugs (GPSCoordinates format, tkhd multi-track, DiskNumber/TrackNumber, BeatsPerMinute, old-style text atoms, HandlerVendorID null, MOV language code) | ✅ Complete | videodecoder_mp4.go, metadecoder_quicktime.go, helpers.go |
| TASK-M11-06 | New requirement tests (HandleTagFieldsPopulated, VideoConfig, Box64BitExtendedSize, BoxSkipUnknown, QuickTimeCreationDateTimezone, EXIFFieldTableSize) | ✅ Complete | videometa_test.go |
| TASK-M11-07 | Real-world benchmarks (exiftool_quicktime.mov, with_audio.mp4) | ✅ Complete | videometa_bench_test.go |
| TASK-M11-08 | Fuzz seed expansion (6 files for FuzzDecodeAllMP4) | ✅ Complete | videometa_fuzz_test.go |

## Milestone 12: Implement Skipped Tests

Every previously-skipped test implemented. No remaining `t.Skip` except conditional file-availability checks.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M12-01 | Fix TestGoldenAppleMOV (QT integer padding, HandlerVendorID from track hdlr) | ✅ Complete | metadecoder_quicktime.go, videodecoder_mp4.go |
| TASK-M12-02 | Remove dead TestGoldenSonyA6700 + orphaned golden JSON | ✅ Complete | videometa_golden_test.go, testdata/ |
| TASK-M12-03 | TestTagsSeparateBySource (REQ-API-16) | ✅ Complete | videometa_test.go |
| TASK-M12-04 | TestBestEffortPartial (REQ-API-18) | ✅ Complete | videometa_test.go |
| TASK-M12-05 | TestBoxExtendToEOF (REQ-BOX-03) + boxEnd overflow fix | ✅ Complete | videometa_test.go, videodecoder_mp4.go, metadecoder_quicktime.go |
| TASK-M12-06 | TestDecodeEXIFAllTypes (REQ-EXIF-03) | ✅ Complete | metadecoder_exif_test.go |
| TASK-M12-07 | TestDecodeIPTCCharsets (REQ-IPTC-02) | ✅ Complete | metadecoder_iptc_test.go |
| TASK-M12-08 | TestDecodeIPTCViaApplicationNotes (REQ-IPTC-04) | ✅ Complete | metadecoder_iptc_test.go, metadecoder_exif_fields.go |
| TASK-M12-09 | TestDecodeXMPExtendedSkip (REQ-XMP-05) | ✅ Complete | metadecoder_xmp_test.go |
---

## Milestone 13: Fix Weak Tests

Comprehensive test audit: strengthen all non-specific assertions, fix fuzz error typing, add latency target test.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M13-P1 | Fix `isInvalidFormatErrorCandidate` to match "allocation too large" errors | ✅ Complete | helpers.go |
| TASK-M13-01 | Fuzz tests assert `IsInvalidFormat` on malformed input errors | ✅ Complete | videometa_fuzz_test.go |
| TASK-M13-02 | Add `TestDecodeLatencyTarget` (REQ-NF-02, < 500us ceiling) | ✅ Complete | videometa_bench_test.go |
| TASK-M13-03 | TestWarnfCallback asserts specific "invalid byte order marker" warning | ✅ Complete | videometa_test.go |
| TASK-M13-04 | TestBestEffortPartial asserts full decode success (not either/or) | ✅ Complete | videometa_test.go |
| TASK-M13-06 | TestBoxSkipUnknown asserts no error + ftyp tag emitted | ✅ Complete | videometa_test.go |
| TASK-M13-07 | TestLimitTagSize three-tier test proving exact > mechanism | ✅ Complete | videometa_test.go |

---

## Milestone 14: Test & Error Robustness

Eliminate fragile string matching in error typing, add per-decoder fuzz targets, expand latency coverage, test large-mdat io.Reader path, add seed corpus regression test.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M14-01 | Replace `isInvalidFormatErrorCandidate` string matching with `stopInvalidFormat` at source | ✅ Complete | io.go, videometa.go, helpers.go |
| TASK-M14-02 | Add FuzzDecodeEXIF, FuzzDecodeXMP, FuzzDecodeIPTC fuzz targets | ✅ Complete | videometa_fuzz_test.go |
| TASK-M14-03 | Expand TestDecodeLatencyTarget to exiftool_quicktime.mov and with_audio.mp4 | ✅ Complete | videometa_bench_test.go |
| TASK-M14-04 | Add TestReaderOnlyLargeMdat for truncated large mdat with non-seekable reader | ✅ Complete | videometa_test.go |
| TASK-M14-05 | Add TestSeedCorpusDecodesSuccessfully regression test for all valid test files | ✅ Complete | videometa_test.go |

---

## Milestone 15: Test & IO Cleanup

Consistency fixes for error typing in IO paths, fuzz target documentation, test file organization.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M15-01 | Make skip() seekable path use stopInvalidFormat for consistency | ✅ Complete | io.go |
| TASK-M15-02 | Document why pos()/seek() use stop() not stopInvalidFormat() | ✅ Complete | io.go |
| TASK-M15-03 | Clarify decoder fuzz targets re: internal recovery and no-error-escape semantics | ✅ Complete | videometa_fuzz_test.go |
| TASK-M15-04 | Move TestSeedCorpusDecodesSuccessfully from bench to test file | ✅ Complete | videometa_bench_test.go, videometa_test.go |
| TASK-M15-05 | Replace `truncated.mp4` fixture coverage with inline malformed-input regressions | ✅ Complete | videometa_test.go, videometa_fuzz_test.go |

---

## Backlog: Extended Test Coverage

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-BL-01 | Android MP4 fixture acquired + local-fixture validation test | ✅ Complete | testdata/google.mp4, videometa_fixture_test.go |
| TASK-BL-02 | GoPro MP4 fixture acquired + local-fixture validation test | ✅ Complete | testdata/gopro_action.mp4, videometa_fixture_test.go |
| TASK-BL-03 | DJI drone MP4 test file + golden test | Pending | testdata/ |
| TASK-BL-04 | Pro camera MOV test file + golden test | Pending | testdata/ |
| TASK-BL-05 | 64-bit box size test file | Pending | testdata/ |
| TASK-BL-06 | Android / Pixel real-file exiftool golden parity (`moov/meta` mdta keys, track metadata) | ✅ Complete | videodecoder_mp4.go, videometa_golden_test.go |
| TASK-BL-07 | GoPro real-file exiftool parity (`udta` vendor boxes, GPMF-derived metadata, timecode metadata) | ✅ Complete | videodecoder_mp4.go, metadecoder_quicktime_gopro.go, videometa_golden_test.go |

---

## Milestone 16: Integrity Recovery

Truthfulness fixes after audit: align docs and validation with the real parser, not the previous claims.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M16-01 | Add `meta/iloc` EXIF/XMP item extraction (`iloc`, `iinf`, `infe`, `pitm`, `idat`) | ✅ Complete | videodecoder_mp4.go, videodecoder_meta_items.go |
| TASK-M16-02 | Add end-to-end tests for UUID EXIF, `meta/iloc` EXIF, and `meta/iloc` XMP | ✅ Complete | videometa_meta_items_test.go |
| TASK-M16-03 | Tighten latency assertion to the documented `<500us` target | ✅ Complete | videometa_bench_test.go |
| TASK-M16-04 | Update requirements, architecture, traceability, and README/CLAUDE status claims to match implementation | ✅ Complete | docs/, README.md, CLAUDE.md |

---

## Milestone 17: Oracle Evidence Hardening

Truthfulness fixes after the integrity recovery increment: generate the full EXIF field reference, harden `idat` extraction, and separate `Validated` from `Implemented` traceability.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M17-01 | Generate EXIF field tables from committed reference manifest (`581/32/5`) | ✅ Complete | gen/, metadecoder_exif_fields.go, exif_fields_reference_test.go |
| TASK-M17-02 | Fix `idat` buffering for non-seekable `meta/iloc` extraction and enforce `idat` extent bounds | ✅ Complete | videodecoder_mp4.go, videodecoder_meta_items.go |
| TASK-M17-03 | Add temp-file exiftool oracle tests for `meta/iloc` EXIF/XMP and XMP UUID routes | ✅ Complete | videometa_oracle_test.go, testhelpers_test.go |
| TASK-M17-04 | Update requirements, architecture, CI, and traceability statuses to distinguish `Validated` from `Implemented` honestly | ✅ Complete | docs/, .github/workflows/ci.yml |

## Milestone 18: NRTM Streaming Guard

Follow-up after PR review: keep Sony NRTM XML `idat` scans bounded by the existing 1 MB XML gate so XML-only decode paths do not allocate large `idat` buffers unnecessarily.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M18-01 | Add public-path regression tests for Sony NRTM XML from `idat` and for large XML-only `idat` skip-on-seek behavior | ✅ Complete | videometa_sony_nrtm_test.go, testhelpers_test.go |
| TASK-M18-02 | Guard Sony NRTM `idat` XML scans behind the `<1MB` direct-scan limit while preserving `meta/iloc` buffering semantics | ✅ Complete | videodecoder_mp4.go |
| TASK-M18-03 | Update architecture/task docs to record the bounded NRTM `idat` scan behavior | ✅ Complete | docs/ARCHITECTURE.md, docs/TASKS.md |

---

## Milestone 19: Scope Cleanup

Refocus the package on video-native metadata paths and remove image-specific vendor EXIF extensions from the public surface.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M19-01 | Remove vendor-specific EXIF extension decoding and API surface | ✅ Complete | videometa.go, metadecoder_exif.go |
| TASK-M19-02 | Reclassify Pentax `TAGS` under vendor container metadata | ✅ Complete | metadecoder_quicktime_pentax.go, videodecoder_mp4.go |
| TASK-M19-03 | Remove legacy tests/fuzzers/docs tied to the retired EXIF vendor extension path | ✅ Complete | *_test.go, docs/, README.md, CLAUDE.md |

---

## Milestone 20: Pre-1.0 API Reset

Reset the public API before 1.0 so namespace identity, vendor taxonomy, and validation status are stable for the long term.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M20-01 | Replace flat collected maps with lossless `Tags`/`SourceTags` preserving source + namespace + tag + decode order | ✅ Complete | videometa.go |
| TASK-M20-02 | Replace `DecodeAll(Options) (Tags, DecodeResult, error)` with `DecodeAll(Options) (Metadata, error)` | ✅ Complete | videometa.go, README.md |
| TASK-M20-03 | Replace public `XML` source with `VENDOR` and move Sony/Pentax families under it | ✅ Complete | videometa.go, videodecoder_mp4.go, metadecoder_quicktime_pentax.go, metadecoder_sony_nrtm.go |
| TASK-M20-04 | Make `Namespace` a stable route identity for QuickTime and vendor container metadata | ✅ Complete | videodecoder_mp4.go, metadecoder_quicktime.go, metadecoder_exif.go |
| TASK-M20-05 | Regress synthetic-only routes from `Validated` to `Implemented` in the requirements matrix | ✅ Complete | docs/REQUIREMENTS.md, requirements_traceability_test.go |

---

## Milestone 21: API Hardening + Evidence Policy

Finish the pre-1.0 surface cleanup: make namespace queries fully lossless, freeze namespace formatting, add streaming allocation guards, and tighten the public support story around real fixtures.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M21-01 | Strengthen traceability enforcement so cited tests must exist, carry `// Validates:` coverage, and `Validated` rows require real-fixture evidence | ✅ Complete | requirements_traceability_test.go, docs/REQUIREMENTS.md |
| TASK-M21-02 | Replace lossy namespace maps with lossless `NamespaceTags`, preserve duplicate tags within a namespace, and add explicit namespace-contract tests using real files | ✅ Complete | videometa.go, videometa_test.go, videodecoder_mp4.go |
| TASK-M21-03 | Add allocation guards for representative streaming-sensitive paths (`mdat`, Sony NRTM `idat`, `meta/iloc`) | ✅ Complete | videometa_alloc_test.go, testhelpers_test.go |
| TASK-M21-04 | Add README examples, support table, and compatibility note that align public claims with validated real-fixture coverage | ✅ Complete | README.md, example_test.go, docs/ARCHITECTURE.md, docs/REQUIREMENTS.md |

---

## Milestone 22: Fixture Acquisition Bootstrap

Record the remaining real-video fixture gaps and make the publicly downloadable ones easy to fetch locally without committing the media.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M22-01 | Add a manifest-driven bootstrap script that downloads every verified official sample fixture into `testdata/` with header/size checks | ✅ Complete | scripts/bootstrap-fixtures.sh, scripts/fixture_bootstrap.tsv |
| TASK-M22-02 | Document the remaining fixture gaps, split them into scriptable vs manual-only, and record source pages plus local target filenames | ✅ Complete | docs/FIXTURE_ACQUISITION.md |
| TASK-M22-03 | Extend `.gitignore` for local-only fixture targets so downloaded media never appears as staged repo content by accident | ✅ Complete | .gitignore |
| TASK-M22-04 | Make the full-service bootstrap restore smaller/high-value fixtures first so GoPro and similar validation assets are not blocked by huge downloads | ✅ Complete | scripts/bootstrap-fixtures.sh, docs/FIXTURE_ACQUISITION.md, CLAUDE.md |

---

## Milestone 23: Golden Validation Hardening

Remove the bogus truncated fixture from the validation corpus and strengthen real-file parity checks so duplicate tags and occurrence order are validated instead of flattened away.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M23-01 | Remove `truncated.mp4` and its golden from the corpus; replace it with inline malformed-input regressions and fuzz seeds | ✅ Complete | videometa_test.go, videometa_fuzz_test.go, testdata/ |
| TASK-M23-02 | Generate duplicate-preserving ordered exiftool goldens alongside grouped JSON goldens | ✅ Complete | gen/main.go, testdata/ |
| TASK-M23-03 | Compare real-file golden tests against both grouped JSON and ordered occurrence goldens | ✅ Complete | videometa_golden_test.go, videometa_golden_ordered_test.go |
| TASK-M23-04 | Update requirements, architecture, CI, and workflow docs to describe the stronger golden system and the removal of `truncated.mp4` from the corpus | ✅ Complete | docs/, README.md, CLAUDE.md, .github/workflows/ci.yml |
