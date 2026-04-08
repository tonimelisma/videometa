# videometa Tasks

Tasks organized by milestone. This file tracks the current product direction, not superseded pre-1.0 experiments.

## Status Summary

| Milestone | Status |
|-----------|--------|
| M1: Foundation + ISOBMFF Parser | ✅ Complete |
| M2: QuickTime + CONFIG | ✅ Complete |
| M3: Vendor Container Metadata | ✅ Complete |
| M4: Lossless Public API | ✅ Complete |
| M5: Real-File Golden Validation | ✅ Complete |
| M6: Fixture Bootstrap + Large Local Corpus | ✅ Complete |
| M7: Video-Native Scope Reset | ✅ Complete |
| M8: Utility Module Refactor | ✅ Complete |
| BL: Remaining Real-Video Fixtures | In progress |

---

## Milestone 1: Foundation + ISOBMFF Parser

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M1-01 | Implement MP4/MOV box parser with `ftyp` validation and recursive descent | ✅ Complete | videodecoder_mp4.go |
| TASK-M1-02 | Extract `VideoConfig` (dimensions, duration, rotation, codec) | ✅ Complete | videodecoder_mp4.go, videometa.go |
| TASK-M1-03 | Support `io.Reader` fallback without buffering whole files | ✅ Complete | io.go, videodecoder_mp4.go |
| TASK-M1-04 | Reject fragmented MP4 (`moof`) | ✅ Complete | videodecoder_mp4.go |

## Milestone 2: QuickTime + CONFIG

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M2-01 | Decode `ilst` atoms and freeform `----` atoms | ✅ Complete | metadecoder_quicktime.go |
| TASK-M2-02 | Decode QuickTime `mdta` / Apple MOV metadata | ✅ Complete | metadecoder_quicktime.go, videodecoder_mp4.go |
| TASK-M2-03 | Decode QuickTime structural tags (`mvhd`, `tkhd`, `gmhd`, `tref`, `stsd` children) | ✅ Complete | videodecoder_mp4.go |
| TASK-M2-04 | Implement `GetDateTime`, `GetLatLong`, and composite tag derivation for supported video metadata | ✅ Complete | videometa.go, datetime.go, gps.go, value.go |

## Milestone 3: Vendor Container Metadata

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M3-01 | Pentax `moov/udta/TAGS` decoder | ✅ Complete | metadecoder_quicktime_pentax.go |
| TASK-M3-02 | Sony UUID-PROF and UUID-USMT/MTDT support | ✅ Complete | videodecoder_mp4.go |
| TASK-M3-03 | Sony NRTM XML parser with repeated-tag preservation | ✅ Complete | metadecoder_sony_nrtm.go |
| TASK-M3-04 | GoPro `udta` / GPMF metadata support | ✅ Complete | metadecoder_quicktime_gopro.go, videodecoder_mp4.go |
| TASK-M3-05 | Emit vendor tags under stable `VENDOR` namespaces | ✅ Complete | videometa.go, videodecoder_mp4.go |

## Milestone 4: Lossless Public API

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M4-01 | Replace lossy flat maps with `Tags` / `SourceTags` / `NamespaceTags` | ✅ Complete | videometa.go |
| TASK-M4-02 | Make `Namespace` a stable route identity | ✅ Complete | videometa.go, videodecoder_mp4.go |
| TASK-M4-03 | Split `QUICKTIME` and `VENDOR` public sources | ✅ Complete | videometa.go |
| TASK-M4-04 | Add namespace-aware duplicate-preservation tests | ✅ Complete | videometa_test.go |

## Milestone 5: Real-File Golden Validation

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M5-01 | Generate grouped exiftool goldens for real fixtures | ✅ Complete | gen/main.go, testdata/ |
| TASK-M5-02 | Add ordered duplicate-preserving exiftool goldens | ✅ Complete | gen/main.go, videometa_golden_ordered_test.go |
| TASK-M5-03 | Validate real fixtures against grouped + ordered goldens | ✅ Complete | videometa_golden_test.go |
| TASK-M5-04 | Enforce golden regeneration in CI | ✅ Complete | .github/workflows/ci.yml |

## Milestone 6: Fixture Bootstrap + Large Local Corpus

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M6-01 | Add manifest-driven bootstrap for public/local-only fixtures | ✅ Complete | scripts/bootstrap-fixtures.sh, scripts/fixture_bootstrap.tsv |
| TASK-M6-02 | Validate Apple, Sony, Google, GoPro, and DJI real fixtures | ✅ Complete | testdata/, videometa_fixture_test.go, videometa_golden_test.go |
| TASK-M6-03 | Document acquisition and gitignored local-fixture workflow | ✅ Complete | docs/FIXTURE_ACQUISITION.md, CLAUDE.md |

## Milestone 7: Video-Native Scope Reset

Refocus the package on video-native metadata only.

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M7-01 | Remove embedded image-metadata decoders and routes (EXIF/XMP/IPTC, `uuid`, `XMP_`, `meta/iloc`) | ✅ Complete | videometa.go, videodecoder_mp4.go |
| TASK-M7-02 | Remove image-metadata tests, fuzzers, and generators | ✅ Complete | *_test.go, gen/main.go |
| TASK-M7-03 | Update requirements, architecture, README, and maintainer docs to declare embedded image metadata an explicit non-goal | ✅ Complete | docs/, README.md, CLAUDE.md |
| TASK-M7-04 | Keep only video-native real-file claims in traceability and README support tables | ✅ Complete | docs/REQUIREMENTS.md, requirements_traceability_test.go |

## Milestone 8: Utility Module Refactor

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-M8-01 | Split the old catch-all helper layer into domain modules for errors, timestamps, GPS, text sanitization, and numeric coercion | ✅ Complete | errors.go, datetime.go, gps.go, text.go, value.go |
| TASK-M8-02 | Rename helper symbols to reflect supported video-metadata behavior and remove test-only runtime helpers | ✅ Complete | videometa.go, videodecoder_mp4.go, metadecoder_quicktime*.go, *_test.go |
| TASK-M8-03 | Fix Sony vendor convenience fallbacks for `CreationDateValue` and name/value GPS pairs | ✅ Complete | videometa.go, gps.go, videometa_test.go |
| TASK-M8-04 | Update architecture, requirements, and maintainer docs to point at the new utility modules | ✅ Complete | docs/, CLAUDE.md |

---

## Backlog: Remaining Real-Video Fixtures

| Task | Description | Status | Files |
|------|-------------|--------|-------|
| TASK-BL-01 | Canon or Panasonic professional camera real fixture | Pending | testdata/, docs/FIXTURE_ACQUISITION.md |
| TASK-BL-02 | 64-bit box-size real or crafted fixture | Pending | testdata/ |
| TASK-BL-03 | Additional validated vendor families only if they are genuinely video/container metadata | Pending | docs/REQUIREMENTS.md |

## Notes

- Embedded image metadata support was deliberately removed before 1.0. Historical EXIF/XMP/IPTC experiments are not part of the current product direction.
- Synthetic media remains acceptable for malformed-input and parser-hardening regressions, but not for README-level support claims.
