# videometa

[![CI](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml/badge.svg)](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tonimelisma/videometa.svg)](https://pkg.go.dev/github.com/tonimelisma/videometa)

Pure Go library for reading metadata from video files. Extracts EXIF, XMP, IPTC, QuickTime native, MakerNotes, and manufacturer-specific metadata from MP4/MOV containers. All output matches `exiftool -n -json`.

## Features

- **Pure Go** — no CGo, no external binaries, `CGO_ENABLED=0` compatible
- **Read-only** — metadata extraction only, no writing
- **MP4/MOV** — ISOBMFF container support (covers ~95% of smartphone video)
- **Multiple sources** — EXIF, XMP, IPTC, QuickTime native metadata, codec/config info
- **Streaming** — never loads entire files into memory; seeks past mdat
- **Validated** — output tested against exiftool via golden files
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

tags, result, err := videometa.DecodeAll(videometa.Options{R: f})
_ = result // Contains VideoConfig (width, height, duration, codec, rotation)

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
| `QUICKTIME` | QuickTime native metadata (ilst atoms, freeform keys) |
| `CONFIG` | Container structure info (dimensions, duration, codec) |
| `MAKERNOTES` | Manufacturer-specific metadata (Pentax TAGS atom) |
| `XML` | Structured XML metadata (Sony NonRealTimeMeta) |

## Benchmarks

```
BenchmarkDecodeMinimalMP4AllSources-8     421803    2839 ns/op    1064 B/op    112 allocs/op
BenchmarkDecodeMinimalMP4ConfigOnly-8     672285    1803 ns/op     608 B/op     78 allocs/op
```

## Status

v0.1.0 — All decoders implemented: ISOBMFF box parser, EXIF, XMP, IPTC, QuickTime native, Pentax MakerNotes, Sony XAVC (UUID-PROF, USMT/MTDT, NRTM XML), Apple MOV (mdta locales, wave/frma). Zero golden file gaps across all test files. Tested with Sony A6700, iPhone 15 Pro, Pentax, and synthetic test files.

## License

MIT
