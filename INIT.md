# videometa — Project Context

## Origin

This project is a Go package for reading metadata from video files, conceived as a companion to [bep/imagemeta](https://github.com/bep/imagemeta). The owner (tonimelisma) is an existing contributor to imagemeta (authored HEIF/HEIC support in #55 and RAW format support in #59).

## Relationship to bep/imagemeta

### What imagemeta provides
- Pure Go library for reading image metadata (EXIF, XMP, IPTC) — writing is explicitly out of scope
- Supported image formats: JPEG, TIFF, PNG, WebP, HEIF/HEIC, AVIF, DNG, CR2, NEF, PEF, ARW
- Output validated against `exiftool -n -json`
- Key internals (all currently **unexported**):
  - `streamReader` — byte-order-aware binary reader with buffering, seeking, position tracking
  - `metaDecoderEXIF` / `newMetaDecoderEXIF` — EXIF IFD parser
  - `decodeXMP` — XMP/RDF XML parser
  - `metaDecoderIPTC` — IPTC record parser
  - ISOBMFF box parser in `imagedecoder_heif.go` (`readBox`, `fourCC`) — used for HEIF/AVIF containers
- Module: `github.com/bep/imagemeta`, requires Go 1.25
- Dependencies: frankban/quicktest, google/go-cmp, rwcarlsen/goexif, golang.org/x/text
- MIT license, authored by Bjørn Erik Pedersen

### Discussion with bep about video support (issue #59)
- bep is **not personally interested** in video metadata currently
- He acknowledged the ISOBMFF overlap between HEIF and MP4/MOV
- He'd be pragmatic about the library name ("video is image frames...")
- Core principle: "maintaining features you don't care about yourself is just work"
- Conclusion: video should live in a separate package

### Code reuse request (issue #64)
Filed https://github.com/bep/imagemeta/issues/64 proposing three options for reusing imagemeta's metadata decoders:

- **A) Export key symbols** — capitalize `streamReader`, `newMetaDecoderEXIF`, `decodeXMP`, `decodeIPTC`, ISOBMFF `readBox`/`fourCC`. Smallest change, expands public API surface.
- **B) Subpackage** — move format-agnostic decoders into `imagemeta/metadecode`. Keeps top-level API unchanged.
- **C) Separate module** — extract decoders into own repo (e.g. `bep/metadecode`). Cleanest dependency graph, most work.

tonimelisma offered to do the refactoring work. Awaiting bep's response.

## Video metadata landscape

### MP4/MOV (priority target)
- ~95% of video is smartphone video in MP4/MOV format
- Uses ISOBMFF box format — same container structure as HEIF/HEIC
- Metadata stored as: EXIF (same as images), XMP (same as images), QuickTime native metadata (new)
- Significant code reuse potential with imagemeta's ISOBMFF parser and EXIF/XMP decoders

### Other formats (lower priority)
- AVI, MKV — essentially zero overlap with imagemeta's code
- Would require completely new container parsers
- Can be added later if needed

## Design principles (inherited from imagemeta)
- Pure Go, no CGo
- Read-only — no writing support
- Validate output against exiftool
- Performance-conscious (streaming, object pooling)
- Fuzz testing included
