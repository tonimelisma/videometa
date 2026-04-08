# videometa

[![CI](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml/badge.svg)](https://github.com/tonimelisma/videometa/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tonimelisma/videometa.svg)](https://pkg.go.dev/github.com/tonimelisma/videometa)

Pure Go library for reading video-native metadata from MP4/MOV files. `videometa` extracts QuickTime/container metadata, vendor container metadata, and codec/config information from ISOBMFF video files. Real-file parity is checked against both grouped `exiftool -n -json -g` goldens and duplicate-preserving ordered exiftool goldens for the supported video metadata paths.

## Features

- **Pure Go**: no CGo, no external binaries, `CGO_ENABLED=0` compatible
- **Read-only**: metadata extraction only, no writing
- **MP4/MOV**: ISOBMFF container support
- **Video-native scope**: QuickTime/container metadata, vendor container metadata, codec/config info
- **Lossless collected API**: tags preserved by source, namespace, tag, and occurrence order
- **Streaming**: never loads entire files into memory; seeks or discards past `mdat`
- **Validated**: README support claims are backed by real-file exiftool goldens
- **Fuzz-tested**: malformed input must not panic

## Explicit Non-Goal

Embedded image metadata payloads are out of scope, even when a video container happens to carry them. `videometa` does **not** parse:

- EXIF/TIFF IFD payloads
- XMP/RDF payloads
- IPTC-IIM payloads
- `uuid`, `XMP_`, `meta/iloc`, or similar carrier routes whose only purpose is embedded image metadata

If a video file contains that kind of payload, use an image-metadata library on the extracted bytes instead. `videometa` intentionally stays focused on video-native/container-native metadata.

## Installation

```bash
go get github.com/tonimelisma/videometa
```

## Usage

### Callback-based streaming decode

```go
f, _ := os.Open("video.mp4")
defer f.Close()

result, err := videometa.Decode(videometa.Options{
    R: f,
    HandleTag: func(ti videometa.TagInfo) error {
        fmt.Printf("%s %s/%s = %v\n", ti.Source, ti.Namespace, ti.Tag, ti.Value)
        return nil
    },
})
fmt.Printf("Video: %dx%d codec=%s\n",
    result.VideoConfig.Width,
    result.VideoConfig.Height,
    result.VideoConfig.Codec)
```

### Collect everything

```go
f, _ := os.Open("video.mp4")
defer f.Close()

metadata, err := videometa.DecodeAll(videometa.Options{R: f})
tags := metadata.Tags

timeScale := tags.QuickTime().Namespace("moov/mvhd").Find("TimeScale")[0]
dt, _ := tags.GetDateTime()
lat, lon, _ := tags.GetLatLong()

_ = timeScale
_ = dt
_ = lat
_ = lon
```

### Filter to the families you want

```go
result, err := videometa.Decode(videometa.Options{
    R:       f,
    Sources: videometa.QUICKTIME | videometa.VENDOR,
    ShouldHandleTag: func(ti videometa.TagInfo) bool {
        return ti.Tag == "Make" || ti.Tag == "Model"
    },
    HandleTag: func(ti videometa.TagInfo) error {
        return nil
    },
})
_ = result
_ = err
```

## Metadata Sources

| Source | Description |
|--------|-------------|
| `QUICKTIME` | Standard/container-native QuickTime metadata |
| `VENDOR` | Vendor-specific container metadata families such as Pentax `TAGS`, Sony UUID/NRTM, and GoPro `udta`/GPMF |
| `CONFIG` | Request flag for `VideoConfig`; not emitted as tags |
| `COMPOSITE` | Derived tags materialized only by `DecodeAll` |

## Namespace Contract

`TagInfo.Namespace` is part of the public API. It is a stable route identity, not a display label.

Examples:

- `ftyp`
- `moov/mvhd`
- `moov/trak[1]/mdia/hdlr`
- `moov/meta/keys`
- `Pentax/moov/udta/TAGS`
- `Sony/meta/nrtm`
- `Sony/uuid/USMT`
- `GoPro/moov/udta/GPMF`

Collected tags are lossless:

- `Tags.All()` preserves decode order
- `SourceTags.Namespace(name).All()` preserves decode order inside one namespace
- repeated tags in the same namespace remain accessible via `Find`, rather than being overwritten

## Support Policy

Public support claims in this README are backed by real video fixtures only.

| Family | Current claim |
|--------|---------------|
| QuickTime/container-native metadata on committed fixtures plus bootstrap-downloadable GoPro/DJI real fixtures | Validated |
| Vendor container metadata on committed fixtures (`Pentax/moov/udta/TAGS`, Sony UUID/NRTM, GoPro `udta`/GPMF) | Validated |
| Additional video-native routes implemented in code but lacking real-fixture evidence | Not promoted to README-level claims until validated |
| Embedded image metadata routes | Explicit non-goal |

## Status

The current release version is recorded in [`VERSION`](VERSION). `videometa` currently implements the ISOBMFF box parser, QuickTime/container-native metadata, vendor metadata families (Pentax `TAGS`, Sony UUID-PROF, Sony USMT/MTDT, Sony NRTM, GoPro `udta`/GPMF), Apple MOV metadata, Android `mdta` metadata, and codec/config extraction. Validation status is real-file-only: Apple, Google, and Sony fixtures are committed in the repo, while GoPro and DJI fixtures are restored by the bootstrap workflow. Synthetic tests remain regression coverage only and do not justify support claims.

## Release Policy

Every merged increment is release-producing.

- `VERSION` is the single source of truth for the next module tag.
- Every PR must bump `VERSION` and add `docs/releases/<VERSION>.md`.
- Releases are cut from `main` only after hosted CI restores the bootstrap-downloadable validated fixtures and passes on the exact merge commit.

## Compatibility Policy

Before `v1.0.0`, the API may change when the model improves. After `v1.0.0`, these become compatibility commitments:

- `Decode`, `DecodeAll`, `Metadata`, `Tags`, `SourceTags`, and `NamespaceTags`
- `Source` names
- tag names
- namespace formats

Validation status in the docs is not an API guarantee and may change as real fixtures are added.

## License

MIT
