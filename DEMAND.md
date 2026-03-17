# Market Demand for Pure Go Video Metadata Extraction

## Executive Summary

No pure Go library exists that extracts rich metadata (EXIF, XMP, IPTC, GPS, camera info, creation dates) from MP4/MOV video files. The demand is demonstrated by: major Go projects (PhotoPrism, 37.8k stars) depending on external Perl/C binaries for video metadata; multiple abandoned/incomplete Go attempts; developers writing one-off gists to parse video timestamps; and a Go ExifTool wrapper (293 stars) existing solely because the pure Go ecosystem falls short.

---

## 1. The Gap: What Exists vs. What's Needed

### What developers need from video metadata

When a camera records an MP4 or MOV file, it embeds metadata in multiple formats within the ISOBMFF container:

- **EXIF**: Camera make/model, lens info, exposure settings, orientation, creation date
- **XMP**: Descriptions, keywords, ratings, GPS coordinates, creator info
- **IPTC**: Copyright, captions, credits, contact info
- **ISOBMFF atoms**: Creation/modification timestamps (mvhd), duration, video dimensions, codec info

Developers building media tools need to extract all of these from a single file, reliably, without external dependencies.

### What pure Go libraries currently provide

| Library | Stars | What it does | What it doesn't do |
|---------|-------|-------------|-------------------|
| [abema/go-mp4](https://github.com/abema/go-mp4) | 531 | Low-level ISOBMFF box I/O | No EXIF/XMP/IPTC extraction |
| [Eyevinn/mp4ff](https://github.com/Eyevinn/mp4ff) | 614 | Full MP4 parsing, streaming focus | No EXIF/XMP/IPTC extraction |
| [alfg/mp4](https://github.com/alfg/mp4) | 132 | Basic moov/mvhd parsing | No EXIF/XMP, limited fields |
| [dhowden/tag](https://github.com/dhowden/tag) | 641 | Audio tags (artist, album, genre) | No video EXIF/GPS/camera data |
| [tajtiattila/metadata](https://github.com/tajtiattila/metadata) | 22 | XMP from MP4, EXIF from JPEG | No EXIF/IPTC from MP4. Abandoned (~2018) |
| [pillash/mp4util](https://github.com/pillash/mp4util) | 15 | MP4 duration only | Nothing else. 5 commits total |
| [bep/imagemeta](https://github.com/bep/imagemeta) | 20 | Full EXIF/XMP/IPTC from images | Images only. Video explicitly excluded |
| [dsoprea/go-exif](https://github.com/dsoprea/go-exif) | 539 | Raw EXIF byte parsing | "Specific image formats are out of scope" |
| [rwcarlsen/goexif](https://github.com/rwcarlsen/goexif) | 666 | EXIF from JPEG/TIFF | No ISOBMFF support. HEIF request open 7+ years |

**The missing piece**: ISOBMFF box parsers exist (go-mp4, mp4ff). Raw EXIF/XMP parsers exist (go-exif, goexif). Nobody has connected them -- built the layer that navigates ISOBMFF containers, locates embedded EXIF/XMP/IPTC data, and feeds it to metadata decoders.

### Non-pure-Go workarounds developers use today

| Library | Stars | Dependency | Penalty |
|---------|-------|-----------|---------|
| [barasher/go-exiftool](https://github.com/barasher/go-exiftool) | 293 | ExifTool binary + Perl runtime | ~50-200ms startup per invocation, deployment complexity |
| [vansante/go-ffprobe](https://github.com/vansante/go-ffprobe) | 201 | FFmpeg/ffprobe binary (~80-150MB) | Massive binary, limited metadata fields |
| [jkl1337/go-mediainfo](https://pkg.go.dev/github.com/jkl1337/go-mediainfo) | -- | libmediainfo via CGo | Breaks cross-compilation, no static binaries |

---

## 2. Specific Demand Evidence

### 2.1 GitHub Issues Requesting Video Metadata in Go

**bep/imagemeta [#64](https://github.com/bep/imagemeta/issues/64)** (March 2026) -- Developer requested access to imagemeta's EXIF/XMP/IPTC decoders and ISOBMFF box parser to build a `videometa` companion package. Three API options proposed. Maintainer declined: "Making it into an API creates extra non-paying work that I have no real incentive doing." Demonstrates direct demand and that the approach of reusing imagemeta was explicitly blocked.

**rwcarlsen/goexif [#70](https://github.com/rwcarlsen/goexif/issues/70)** (January 2019, open 7+ years) -- User reported "HEIF stores EXIF into mp4 box/atom container" and asked for support. Maintainer acknowledged it "should become a high priority" but never implemented it. HEIF uses the same ISOBMFF container as MP4/MOV -- same gap.

**dsoprea/go-exif [#8](https://github.com/dsoprea/go-exif/issues/8)** (February 2019) -- User asked about HEIF/ISOBMFF EXIF support. Maintainer: "Specific image formats are out of scope for this project. It requires you to provide the raw EXIF bytes." Highlights the fundamental problem: raw EXIF parsers exist but nobody extracts those bytes from ISOBMFF containers.

**PhotoPrism [#501](https://github.com/photoprism/photoprism/issues/501)** (September 2020, 5 thumbs up, still open) -- Feature request to evaluate ffprobe as an alternative to ExifTool for video metadata. Discussion acknowledges neither is a pure Go solution. The most popular Go photo manager (37.8k stars) cannot natively parse video metadata.

**PhotoPrism [#810](https://github.com/photoprism/photoprism/issues/810)** (January 2021, 10 comments) -- Users reported MP4 videos sorted by file modification date instead of embedded creation date. Root cause: the importer doesn't run ExifTool before moving files. Had to add an ExifTool call to fix it. A pure Go library would have made this trivial.

**Photoview [#894](https://github.com/photoview/photoview/issues/894)** (October 2023, 3 comments) -- Request to display GPS coordinates from video files on a map. Suggested workaround: shell out to `exiftool -location:all` or `ffmpeg -i input.mp4 -f ffmetadata -`.

**Photoview [#787](https://github.com/photoview/photoview/issues/787)** (January 2023, 5 comments) -- Photoview's Go backend failed because the exiftool binary wasn't in `$PATH` on Arch Linux. The fragility of the external binary dependency approach.

**simulot/immich-go [#332](https://github.com/simulot/immich-go/issues/332)** (June 2024, 5 comments) -- MP4 creation timestamps extracted incorrectly (year 221014 instead of 2016). immich-go implements its own minimal mvhd reader in a 32KB buffer -- the limited pure Go approach leads to bugs.

**perkeep/perkeep [#745](https://github.com/perkeep/perkeep/issues/745)** (April 2016, 4 comments) -- Requested MP4 metadata indexing for Perkeep (by Brad Fitzpatrick). Wanted creation dates and metadata for organizing video files alongside photos.

**perkeep/perkeep [#778](https://github.com/perkeep/perkeep/issues/778)** (May 2016, 19 comments) -- Substantial discussion on mapping file metadata (EXIF, GPS, ID3) to permanode attributes, including from video files.

**Hugo [#10151](https://github.com/gohugoio/hugo/issues/10151)** (August 2022, 3 thumbs up, 3 hearts) -- Requested expanding Hugo's resource metadata to audio files, drawing parallels to EXIF extraction. Closed as not planned. No equivalent video metadata request exists, but the pattern is established.

### 2.2 Developers Writing One-Off Parsers

**[phelian's Gist](https://gist.github.com/phelian/81bbb30cd78aceb05c8d467243edb217)** (8 stars, 4 revisions, active comments) -- A standalone Go program that manually parses MOV/MP4 binary structure to find the movie header atom and extract creation time using Apple's 1904 epoch. A commenter built a "mediaRenamerToTimestamp" project from it. Developers are hand-parsing binary formats because no library exists.

**immich-go's built-in reader** ([source](https://deepwiki.com/simulot/immich-go/4.1-metadata-extraction)) -- Implements its own minimal MP4 reader: searches for the `mvhd` atom in a 32KB buffer, calls `decodeMvhdAtom`, falls back to file mtime if timestamp is before year 2000. Only extracts creation time. No EXIF, XMP, GPS, camera info.

### 2.3 Major Go Projects Depending on External Binaries

**PhotoPrism** (37.8k stars) -- Uses `dsoprea/go-exif` for images but requires ExifTool (Perl) for video metadata. Config flags `PHOTOPRISM_DISABLE_EXIFTOOL` and `PHOTOPRISM_DISABLE_FFMPEG` exist because these are known deployment pain points. Metadata extraction follows a fallback chain: EXIF -> XMP -> JSON (ExifTool) -> filename -> filesystem mtime.

**Photoview** -- Go backend uses `go-exiftool` wrapper, requiring the ExifTool binary in `$PATH`. Users have reported deployment failures when ExifTool is missing or misconfigured.

**img-sort** ([patrickap/img-sort](https://github.com/patrickap/img-sort)) -- Go tool for organizing photos/videos by date. Requires Perl 5+ and ExifTool 12.55+ as external dependencies for MP4/MOV date extraction.

**go-media-organizer** ([allanavelar/go-media-organizer](https://github.com/allanavelar/go-media-organizer)) -- Go CLI for organizing media by date. Requires both ExifTool and FFmpeg.

### 2.4 The CGo Removal Trend in Go Media Projects

The audio metadata ecosystem just went through the exact transition that video metadata needs:

**Navidrome** (19.6k stars) -- [PR #4902](https://github.com/navidrome/navidrome/pull/4902) introduced `sentriz/go-taglib` as a pure Go metadata extractor, replacing the CGo-based TagLib dependency. Described as "a significant step toward removing the C++ TagLib dependency" to "simplify cross-platform builds and packaging."

**Gonic** (2.3k stars) -- "Gonic now vendors reproducible WebAssembly backends for TagLib and SQLite, eliminating CGo and external system libraries." Uses [sentriz/go-taglib](https://github.com/sentriz/go-taglib) (77 stars).

Both projects invested significant effort to eliminate CGo for audio metadata. Video metadata is the next logical frontier, and no equivalent solution exists.

---

## 3. Use Cases Requiring Pure Go

### 3.1 Serverless / AWS Lambda

ExifTool on Lambda requires a [Perl Lambda layer + ExifTool layer](https://codegyver.com/2022/08/22/exiftool-aws-lambda/). FFmpeg requires an [80-150MB binary layer](https://aws.amazon.com/blogs/compute/extracting-video-metadata-using-lambda-and-mediainfo/). Lambda has a 250MB uncompressed limit. Every dependency increases cold start time.

A pure Go binary needs zero layers, ships under 20MB, and cold-starts in 200-400ms.

### 3.2 WebAssembly / WASM

Pure Go compiles to WASM (`GOOS=js GOARCH=wasm` since Go 1.11, official WASI support since Go 1.21). CGo-based libraries cannot compile to WASM. Use cases:

- Browser-based video metadata extraction (privacy-preserving, no server upload)
- Edge computing (Cloudflare Workers, Fastly Compute)
- WASI-based plugin systems

### 3.3 Cross-Compilation

CGo is disabled by default during cross-compilation ([Go issue #4714](https://github.com/golang/go/issues/4714), open since 2013). Using CGo cross-platform requires Docker toolchains and per-platform C headers.

Pure Go: `GOOS=linux GOARCH=arm64 go build` just works.

### 3.4 Static Binaries / Docker Image Size

A pure Go binary deploys in a `FROM scratch` Docker image (~6-10MB). PhotoPrism's Docker image is ~200MB compressed due to ExifTool, FFmpeg, and other runtime dependencies.

### 3.5 Embedded / IoT

Raspberry Pi, NAS appliances, and other constrained devices benefit from a single static binary without Perl, FFmpeg, or shared library requirements.

---

## 4. Performance Comparison

| Approach | Per-file latency | Notes |
|----------|-----------------|-------|
| ExifTool (no stay_open) | ~50-200ms | Perl process startup per invocation |
| ExifTool (stay_open) | ~6-7ms | Persistent process, IPC overhead |
| go-exiftool wrapper | ~6-7ms + Go overhead | Best case with warm ExifTool process |
| Native Go (bep/imagemeta) | ~8-30 microseconds | EXIF from JPEG/PNG, for reference |
| WASM approach (go-taglib) | ~0.3ms | Audio metadata, for reference |
| **Expected: native Go video** | **~50-500 microseconds** | **Box navigation + EXIF/XMP decode** |

Native Go parsing is **20-100x faster** than shelling out to ExifTool even with `-stay_open` optimization. For batch processing (photo libraries with thousands of videos), this is the difference between seconds and minutes.

---

## 5. Hugo Ecosystem Gap

Hugo currently:
- Extracts EXIF, XMP, and IPTC from images via `.Meta` / `.Exif` (JPEG, PNG, TIFF, WebP, AVIF, HEIC/HEIF)
- Detects video media types (MP4, MPEG, AVI, WEBM, OGG, 3GPP) via `resources.ByType "video"`
- **Cannot extract any metadata from video files** -- no creation dates, no GPS, no camera info

Hugo users building photo/video galleries sort images by EXIF date but have no equivalent capability for video. Videos are opaque blobs in Hugo's resource system. A `videometa` library following imagemeta's patterns would be a natural complement, though bep has [declined to expose imagemeta's internals](https://github.com/bep/imagemeta/issues/64) for reuse.

---

## 6. Why Previous Attempts Failed or Stalled

| Project | Why it stalled |
|---------|---------------|
| tajtiattila/metadata | Scope too broad (JPEG + MP4), only got MP4 XMP working, never implemented EXIF from ISOBMFF. Abandoned ~2018 |
| pillash/mp4util | Too narrow (duration only). Apparently a weekend project that was never expanded |
| alfg/mp4 | Structural parser, never intended to extract photographer/camera metadata |
| abema/go-mp4, Eyevinn/mp4ff | Focused on streaming/encoding use cases, not metadata extraction. Low-level by design |
| dhowden/tag | Audio metadata library. MP4 support is for iTunes/AAC tags, not video EXIF |

The pattern: projects either built box parsers (useful for streaming) or attempted full metadata extraction (too ambitious, stalled). Nobody built the focused middle layer: ISOBMFF navigation specifically for locating and extracting EXIF/XMP/IPTC embedded in video files.

---

## 7. Addressable Market Summary

### Direct consumers (Go projects that need video metadata today)

| Project | Stars | Current approach | Would benefit |
|---------|-------|-----------------|--------------|
| PhotoPrism | 37,800 | ExifTool binary | Eliminate Perl dependency |
| Navidrome | 19,600 | Already removed CGo for audio | Pattern validates demand |
| immich-go | ~5,000 | Minimal hand-rolled mvhd parser | Replace fragile bespoke code |
| Gonic | 2,300 | Removed CGo for audio | Pattern validates demand |
| Photoview | ~5,000 | go-exiftool wrapper | Eliminate ExifTool dependency |
| go-exiftool users | 293 (wrapper stars) | ExifTool binary | Pure Go alternative |
| go-ffprobe users | 201 (wrapper stars) | FFmpeg binary | Pure Go alternative for metadata |
| Hugo | 79,000+ | No video metadata at all | Enable video EXIF/GPS/dates |

### Indirect demand signals

- 666 stars on rwcarlsen/goexif (image EXIF) -- established demand for Go metadata parsing
- 539 stars on dsoprea/go-exif (raw EXIF) -- same
- 641 stars on dhowden/tag (audio tags from MP4) -- developers want metadata from MP4 containers
- 614 stars on Eyevinn/mp4ff + 531 on abema/go-mp4 -- developers work with MP4 in pure Go
- Multiple one-off gists and Stack Overflow snippets for parsing MP4 timestamps in Go

---

## 8. Conclusion

The demand is not speculative. Major Go projects depend on external Perl/C binaries for video metadata extraction today. Multiple developers have tried and failed to build a complete pure Go solution. The audio metadata ecosystem has already gone through the CGo-removal transition, validating the pattern. The ISOBMFF box parsers and raw EXIF decoders exist separately in Go -- the missing piece is the metadata extraction layer that connects them for MP4/MOV video files.

A well-built `videometa` library would fill a gap that has been open for at least 7 years (rwcarlsen/goexif HEIF issue, 2019) and serve projects with a combined 130k+ GitHub stars.
