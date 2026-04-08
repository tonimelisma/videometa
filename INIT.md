# Project History

`videometa` started as a companion project to `bep/imagemeta`, but the scope has since been reset.

## Original Exploration

The earliest prototype explored a broad “extract whatever metadata a video container might carry” direction. That included ideas around embedded image-metadata payloads as well as video-native metadata.

## Current Direction

The current product boundary is intentionally narrower and cleaner:

- `videometa` handles video-native/container-native metadata from MP4/MOV
- `videometa` handles vendor video metadata families
- `videometa` does **not** parse embedded image metadata payloads

That reset happened before 1.0 specifically so the package could keep a coherent API and a scope we can live with long term.

## Architectural Consequence

The durable value of this package is:

1. ISOBMFF navigation
2. video-specific routing
3. lossless namespace-aware metadata collection
4. real-fixture parity with exiftool on supported video metadata

If a future workflow needs EXIF/TIFF, XMP/RDF, or IPTC-IIM payload parsing, that should live in a separate image-metadata library and not expand `videometa` back across that boundary.
