# videometa

[![CI](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml/badge.svg)](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tonimelisma/videometa.svg)](https://pkg.go.dev/github.com/tonimelisma/videometa)

Pure Go library for reading metadata from video files. Extracts EXIF, XMP, IPTC, QuickTime native, and vendor-specific container metadata from MP4/MOV files. Real-file parity is checked against both grouped `exiftool -n -json -g` goldens and duplicate-preserving ordered exiftool goldens for the supported video metadata paths.

## Features

- **Pure Go** — no CGo, no external binaries, `CGO_ENABLED=0` compatible
- **Read-only** — metadata extraction only, no writing
- **MP4/MOV** — ISOBMFF container support (covers ~95% of smartphone video)
- **Multiple sources** — EXIF, XMP, IPTC, QuickTime native metadata, vendor container metadata, codec/config info
- **Streaming** — never loads entire files into memory; seeks past mdat
- **Validated** — public support claims are backed by real-file exiftool goldens
- **Fuzz-tested** — no panics on malformed input

## Installation

```
go get github.com/tonimelisma/videometa
```

## Usage

### Callback-based (streaming)

```go
f, _ := os.Open("video.mp4")
defer f.Close()

result, err := videometa.Decode(videometa.Options{
    R: f,
    HandleTag: func(ti videometa.TagInfo) error {
        fmt.Printf("%s/%s = %v\n", ti.Source, ti.Tag, ti.Value)
        return nil
    },
})
fmt.Printf("Video: %dx%d, codec=%s\n",
    result.VideoConfig.Width,
    result.VideoConfig.Height,
    result.VideoConfig.Codec)
```

### Convenience (collect all tags)

```go
f, _ := os.Open("video.mp4")
defer f.Close()

metadata, err := videometa.DecodeAll(videometa.Options{R: f})
tags := metadata.Tags
_ = metadata.VideoConfig // Contains width, height, duration, codec, rotation

// Namespace-aware access: no lossy flattening.
timeScale := tags.QuickTime().Namespace("moov/mvhd").Find("TimeScale")[0]

// Get creation time (priority: EXIF > XMP > QuickTime > IPTC)
dt, _ := tags.GetDateTime()

// Get GPS coordinates (priority: EXIF > XMP > QuickTime)
lat, lon, _ := tags.GetLatLong()
```

### Filtering

```go
result, err := videometa.Decode(videometa.Options{
    R:       f,
    Sources: videometa.EXIF | videometa.QUICKTIME, // Skip XMP/IPTC
    ShouldHandleTag: func(ti videometa.TagInfo) bool {
        return ti.Tag == "Make" || ti.Tag == "Model"
    },
    HandleTag: func(ti videometa.TagInfo) error {
        // Only Make and Model tags arrive here.
        return nil
    },
})
```

## Metadata Sources

| Source | Description |
|--------|------------|
| `EXIF` | EXIF IFD data (camera info, GPS, exposure) |
| `XMP` | XMP/RDF XML metadata |
| `IPTC` | IPTC-IIM records (keywords, captions) |
| `QUICKTIME` | Standard/container-native QuickTime metadata |
| `VENDOR` | Vendor-specific container metadata families (Pentax `TAGS`, Sony UUID/NRTM, GoPro `udta`/GPMF) |
| `CONFIG` | Request flag for `VideoConfig`; not emitted as tags |
| `COMPOSITE` | Derived tags materialized only by `DecodeAll` |

## Namespace Contract

`TagInfo.Namespace` is part of the public API. It is a stable route identity, not a display label.

Examples:

- `moov/mvhd`
- `moov/trak[1]/mdia/hdlr`
- `moov/meta/keys`
- `Pentax/moov/udta/TAGS`
- `Sony/uuid/USMT`

Collected tags are lossless:

- `Tags.All()` preserves decode order
- `SourceTags.Namespace(name).All()` preserves decode order inside one namespace
- repeated tags in the same namespace remain accessible via `Find`, rather than being overwritten

## Support Policy

Public support claims in this README are backed by real video fixtures only.

| Family | Current claim |
|--------|---------------|
| QuickTime/container-native metadata on committed fixtures and documented local Apple/Sony/Google/GoPro real fixtures | Validated |
| Vendor container metadata on committed fixtures (`Pentax/moov/udta/TAGS`, Sony UUID/NRTM, Apple MOV mdta, GoPro `udta`/GPMF) | Validated |
| Additional embedded routes implemented in code and regression tests | Not promoted to README-level claims until real-fixture validation exists |

## Benchmarks

```
BenchmarkDecodeMinimalMP4AllSources-8     421803    2839 ns/op    1064 B/op    112 allocs/op
BenchmarkDecodeMinimalMP4ConfigOnly-8     672285    1803 ns/op     608 B/op     78 allocs/op
```

## Status

v0.1.0 development snapshot — ISOBMFF box parser, EXIF, XMP, IPTC, QuickTime native, vendor metadata families (Pentax `TAGS`, Sony UUID-PROF, Sony USMT/MTDT, Sony NRTM, GoPro `udta`/GPMF), Apple MOV metadata, and Android `mdta` metadata are implemented. Validation status is real-file-only; synthetic tests remain regression coverage only and do not justify support claims.

## Compatibility Policy

Before `v1.0.0`, the API may change when the model improves. After `v1.0.0`, these become compatibility commitments:

- `Decode`, `DecodeAll`, `Metadata`, `Tags`, `SourceTags`, and `NamespaceTags`
- `Source` names
- tag names
- namespace formats

Validation status in the docs is not an API guarantee and may change as real fixtures are added.

## License

MIT
