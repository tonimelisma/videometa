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

---

## Milestone 1–8: Core Implementation

All complete. See git history for details.

---

## Milestone 9: Manufacturer-Specific Video Metadata

Reframed from "EXIF MakerNotes" to cover manufacturer-specific video container metadata.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M9-01 | Pentax TAGS binary parser (7 tags: Make, ExposureTime, FNumber, ExposureCompensation, WhiteBalance, FocalLength, ISO) | ✅ Complete | metadecoder_makernotes_pentax.go |
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
| TASK-M11-03 | Fix nerfed tests (TestDecodeTruncatedMP4, TestWarnfCallback, TestDecodeTimeout) | ✅ Complete | videometa_test.go |
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
| TASK-M12-10 | TestDecodeMakerNotesRouting (REQ-EXIF-07/08/09) | ✅ Complete | videometa_test.go, testhelpers_test.go |

---

## Milestone 13: Fix Weak Tests

Comprehensive test audit: strengthen all non-specific assertions, fix fuzz error typing, add latency target test.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M13-P1 | Fix `isInvalidFormatErrorCandidate` to match "allocation too large" errors | ✅ Complete | helpers.go |
| TASK-M13-01 | Fuzz tests assert `IsInvalidFormat` on malformed input errors | ✅ Complete | videometa_fuzz_test.go |
| TASK-M13-02 | Add `TestDecodeLatencyTarget` (REQ-NF-02, < 2ms ceiling) | ✅ Complete | videometa_bench_test.go |
| TASK-M13-03 | TestWarnfCallback asserts specific "invalid byte order marker" warning | ✅ Complete | videometa_test.go |
| TASK-M13-04 | TestBestEffortPartial asserts full decode success (not either/or) | ✅ Complete | videometa_test.go |
| TASK-M13-05 | TestDecodeMakerNotesRouting asserts warning content + no MAKERNOTES tags | ✅ Complete | videometa_test.go |
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
| TASK-M14-05 | Add TestSeedCorpusDecodesSuccessfully regression test for all valid test files | ✅ Complete | videometa_bench_test.go |

---

## Backlog: Extended Test Coverage

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-BL-01 | Android MP4 test file + golden test | Pending | testdata/ |
| TASK-BL-02 | GoPro MP4 test file + golden test | Pending | testdata/ |
| TASK-BL-03 | DJI drone MP4 test file + golden test | Pending | testdata/ |
| TASK-BL-04 | Pro camera MOV test file + golden test | Pending | testdata/ |
| TASK-BL-05 | 64-bit box size test file | Pending | testdata/ |
| TASK-BL-06 | EXIF MakerNotes (Apple, Canon, Sony in EXIF IFD) | Pending | metadecoder_makernotes_*.go |
