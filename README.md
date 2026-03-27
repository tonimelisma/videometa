# videometa

[![CI](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml/badge.svg)](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tonimelisma/videometa.svg)](https://pkg.go.dev/github.com/tonimelisma/videometa)

Pure Go library for reading metadata from video files. Extracts EXIF, XMP, IPTC, QuickTime native, and vendor-specific container metadata from MP4/MOV files. All output matches `exiftool -n -json` for the supported video metadata paths.

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
| `VENDOR` | Vendor-specific container metadata families (Pentax `TAGS`, Sony UUID/NRTM) |
| `CONFIG` | Request flag for `VideoConfig`; not emitted as tags |
| `COMPOSITE` | Derived tags materialized only by `DecodeAll` |

## Benchmarks

```
BenchmarkDecodeMinimalMP4AllSources-8     421803    2839 ns/op    1064 B/op    112 allocs/op
BenchmarkDecodeMinimalMP4ConfigOnly-8     672285    1803 ns/op     608 B/op     78 allocs/op
```

## Status

v0.1.0 development snapshot — ISOBMFF box parser, EXIF, XMP, IPTC, QuickTime native, vendor metadata families (Pentax `TAGS`, Sony UUID-PROF, Sony USMT/MTDT, Sony NRTM), and Apple MOV metadata are implemented. Validation status is real-file-only; synthetic tests remain regression coverage only and do not justify support claims.

## License

MIT
