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
| M11: Extended Test Coverage | Future work |

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

## Milestone 11: Extended Test Coverage (post-v0.1.0)

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M11-01 | Android MP4 test file + golden test | Pending | testdata/ |
| TASK-M11-02 | GoPro MP4 test file + golden test | Pending | testdata/ |
| TASK-M11-03 | DJI drone MP4 test file + golden test | Pending | testdata/ |
| TASK-M11-04 | Pro camera MOV test file + golden test | Pending | testdata/ |
| TASK-M11-05 | 64-bit box size test file | Pending | testdata/ |
| TASK-M11-06 | EXIF MakerNotes (Apple, Canon, Sony in EXIF IFD) | Pending | metadecoder_makernotes_*.go |
