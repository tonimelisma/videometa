# videometa Architecture

Each design element has an ID (`ARCH-*`) linked back to requirements (`REQ-*`).

---

## 1. Data Flow (`ARCH-FLOW-*`)

| ID | Description | Traces to |
|----|-------------|-----------|
| ARCH-FLOW-01 | End-to-end pipeline from reader through box parser to metadata decoders and callbacks | REQ-API-01..22 |

```text
io.ReadSeeker (or io.Reader fallback)
  -> streamReader
    -> ISOBMFF box parser (videodecoder_mp4.go)
      -> metadata router (box path -> decoder dispatch)
        -> QuickTime decoder -> HandleTag callback
        -> Vendor decoder families -> HandleTag callback
        -> CONFIG extractor -> DecodeResult.VideoConfig
        -> DecodeAll collector -> Metadata{Tags, VideoConfig}
```

Embedded image metadata payloads are intentionally not part of this pipeline. `videometa` does not route container-carried EXIF/XMP/IPTC payloads into dedicated decoders.

---

## 2. File Layout (`ARCH-FILE-*`)

| ID | File | Purpose | Traces to |
|----|------|---------|-----------|
| ARCH-FILE-01 | `videometa.go` | Public API: Decode, DecodeAll, Metadata, Options, TagInfo, Tags, SourceTags, NamespaceTags, Source, DecodeResult, VideoConfig | REQ-API-* |
| ARCH-FILE-02 | `io.go` | `streamReader`: binary reads, seek/discard, panic control flow, position tracking | REQ-NF-01, REQ-NF-02 |
| ARCH-FILE-03 | `videodecoder_mp4.go` | ISOBMFF parser, routing, CONFIG extraction, sample-description child-atom handling, UUID/vendor dispatch, bounded Sony NRTM `idat` scans | REQ-BOX-*, REQ-CFG-* |
| ARCH-FILE-04 | `metadecoder_quicktime.go` | QuickTime `ilst`/freeform/mdta parser, locale handling, tag name tables | REQ-QT-* |
| ARCH-FILE-05 | `metadecoder_quicktime_pentax.go` | Pentax `TAGS` vendor metadata family | REQ-VENDOR-* |
| ARCH-FILE-06 | `metadecoder_quicktime_gopro.go` | GoPro `udta`/GPMF vendor metadata family | REQ-VENDOR-* |
| ARCH-FILE-07 | `metadecoder_sony_nrtm.go` | Sony NRTM XML vendor metadata parser | REQ-VENDOR-* |
| ARCH-FILE-08 | `helpers.go` | InvalidFormatError, time parsing, ISO6709 parsing, shared formatting helpers | REQ-QT-06, REQ-NF-06 |
| ARCH-FILE-09 | `gen/main.go` | Golden file generator (grouped JSON + ordered occurrence goldens) | REQ-NF-04 |
| ARCH-FILE-10 | `testdata/` | Test video files + grouped and ordered exiftool goldens | REQ-TEST-* |
| ARCH-FILE-11 | `.github/workflows/ci.yml` | CI with exiftool-backed golden validation | REQ-NF-10 |

---

## 3. ISOBMFF Box Parser (`ARCH-BOX-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|-----------|
| ARCH-BOX-01 | Recursive descent over known containers (`moov`, `trak`, `mdia`, `minf`, `stbl`, `udta`, `meta`) with bounded depth | Keeps routing local to each container while preserving streaming behavior | REQ-BOX-01..08, REQ-NF-06 |
| ARCH-BOX-02 | `readBoxHeader()` returns `(totalSize, fourcc, isEOF)` | Core primitive for all navigation | REQ-BOX-01..04 |
| ARCH-BOX-03 | Skip `mdat` by seeking (`ReadSeeker`) or read+discard (`Reader`) | `mdat` can be gigabytes | REQ-BOX-05, REQ-API-03 |
| ARCH-BOX-04 | Route only supported video-native metadata-bearing boxes and vendor UUID families | Keeps the package aligned with its explicit scope; embedded image metadata routes are intentionally ignored | REQ-BOX-07, REQ-BOX-08 |
| ARCH-BOX-05 | `ftyp` validation checks major and compatible brands | Detect MOV vs MP4 internally | REQ-BOX-06, REQ-API-04 |

### Box Path Routing Table

| Box path | Action |
|----------|--------|
| `ftyp` | Validate brand, set MOV/MP4 mode |
| `moov/mvhd` | -> CONFIG: timestamps, duration, timescale |
| `moov/trak/tkhd` | -> CONFIG: dimensions, rotation |
| `moov/trak/mdia/minf/stbl/stsd` | -> CONFIG: codec fourcc + validated child atoms (`fiel`, `colr`, `pasp`, `btrt`) |
| `moov/udta/meta` | Parse as FullBox, descend |
| `moov/udta/meta/ilst` | -> QuickTime decoder |
| `moov/udta/meta/ilst/----` | -> QuickTime freeform decoder |
| `moov/udta/TAGS` | -> Pentax vendor decoder |
| `moov/udta/FIRM`, `LENS`, `CAME`, `MUID`, `GPMF` | -> GoPro vendor decoder |
| `moov/udta/©xxx` | -> QuickTime old-style text atoms |
| `moov/trak/tref/cdsc` | -> `ContentDescribes` |
| `moov/trak/mdia/minf/gmhd/gmin` | -> generic media header tags |
| `moov/trak/meta` | -> mdta keys/ilst (track-level metadata) |
| `moov/meta (hdlr=nrtm)` | -> Sony NRTM (`xml ` box or bounded `idat` scan) |
| `uuid (PROF GUID)` | -> Sony XAVC profile |
| `uuid (USMT GUID)` | -> Sony MTDT / TrackProperty / TimeZone |
| `moof` | -> return fragmented MP4 error |
| Other | Skip silently |

### Explicit Non-Goal

These routes are intentionally ignored:

- `moov/udta/XMP_`
- embedded EXIF/XMP carrier UUIDs
- `meta/iloc` item extraction whose only payloads are embedded image metadata

They are not “temporarily unsupported”; they are out of scope by design.

---

## 4. Metadata Decoders (`ARCH-DEC-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|-----------|
| ARCH-DEC-01 | QuickTime decoder: `ilst`, freeform, mdta, locale handling, exiftool-exact tag names | No image-metadata dependency; core video/container logic | REQ-QT-* |
| ARCH-DEC-02 | Vendor decoders: Pentax `TAGS`, Sony UUID/NRTM, GoPro `udta`/GPMF | Vendor metadata is still video/container metadata and remains first-class | REQ-VENDOR-* |
| ARCH-DEC-03 | Lossless collector stores tags by source + namespace + tag + occurrence order, exposed via `SourceTags` and `NamespaceTags` | Prevents collisions and silent overwrites | REQ-API-16, REQ-API-19, REQ-API-22 |
| ARCH-DEC-04 | Shared helpers parse video-native timestamps and ISO6709 GPS coordinates | Keeps API conveniences aligned with supported metadata | REQ-API-11, REQ-API-13, REQ-QT-06 |

### Namespace Contract

- QuickTime/container namespaces are exact route identities with 1-based track ordinals where needed, e.g. `moov/trak[1]/mdia/hdlr`
- Vendor namespaces use `VendorName/route`, e.g. `Pentax/moov/udta/TAGS`, `Sony/uuid/USMT`, `Sony/meta/nrtm`, `GoPro/moov/udta/GPMF`
- `SourceTags.Namespace(name)` returns a lossless `NamespaceTags` view
- Repeated tags inside one namespace remain queryable in decode order via `NamespaceTags.Find()` and `SourceTags.FindInNamespace()`

---

## 5. streamReader (`ARCH-IO-*`)

| ID | Design element | Rationale | Traces to |
|----|----------------|-----------|
| ARCH-IO-01 | Wraps `io.ReadSeeker`/`io.Reader` with convenience binary reads and position tracking | REQ-NF-01 |
| ARCH-IO-03 | Panic-based internal control flow (`panic(errStop)` on EOF) recovered at the public boundary | REQ-NF-01 |
| ARCH-IO-04 | Buffer-pool usage and bounded helpers keep allocations low | REQ-NF-02 |

ISOBMFF parsing is always big-endian. Vendor sub-decoders parse their own payload formats locally where needed.

---

## 6. Error Handling (`ARCH-ERR-*`)

| ID | Design element | Traces to |
|----|----------------|-----------|
| ARCH-ERR-01 | `InvalidFormatError` for malformed input | REQ-NF-06 |
| ARCH-ERR-02 | `ErrStopWalking` for caller-initiated early termination | REQ-API-15 |
| ARCH-ERR-04 | Timeout via goroutine + channel | REQ-API-10 |
| ARCH-ERR-05 | Partial failure on supported paths: malformed Sony XML or vendor payloads warn and skip without aborting the whole decode | REQ-API-09, REQ-API-17, REQ-API-18 |

---

## 7. Testing Architecture (`ARCH-TEST-*`)

| ID | Design element | Traces to |
|----|----------------|-----------|
| ARCH-TEST-01 | `go generate ./gen` runs exiftool on committed/local real test videos and regenerates grouped JSON + ordered occurrence goldens for supported groups only | REQ-NF-04 |
| ARCH-TEST-02 | Public validation compares videometa output against committed real-file grouped JSON goldens and duplicate-preserving ordered occurrence goldens | REQ-NF-04, REQ-VENDOR-04 |
| ARCH-TEST-03 | Benchmarks and latency guards cover representative streaming-sensitive paths | REQ-NF-02, REQ-NF-03 |
| ARCH-TEST-04 | Fuzz and malformed-input tests cover parser safety and robustness | REQ-NF-05, REQ-NF-06 |
| ARCH-TEST-05 | CI reruns `go generate ./gen` and diffs the grouped and ordered goldens | REQ-NF-10 |

Large local fixtures remain gitignored but are part of the real validation corpus when present.

---

## 8. Dependencies (`ARCH-DEP-*`)

| ID | Dependency | Type | Purpose | Traces to |
|----|------------|------|---------|-----------|
| ARCH-DEP-01 | Runtime dependencies | None | Pure-Go runtime with stdlib-only metadata decoding | REQ-NF-08 |
| ARCH-DEP-02 | frankban/quicktest | Test | Assertions | — |
| ARCH-DEP-03 | google/go-cmp | Test | Deep comparison helpers | — |
