# videometa Demand Notes

This document records the product case for a pure-Go video metadata library focused on **video-native/container-native** metadata.

## Problem

Go projects that need metadata from MP4/MOV video files still end up depending on external tools for metadata that lives in:

- QuickTime/container-native metadata
- Apple `mdta` / freeform metadata
- vendor container metadata such as Sony UUID/NRTM and GoPro `udta`/GPMF
- codec/config metadata needed for indexing and organization

The gap is not “raw metadata parsing in general.” Go already has mature image-metadata libraries. The gap is a focused library that can navigate ISOBMFF containers and emit the video-native metadata exiftool already knows how to show.

## What `videometa` Is For

- MP4/MOV container traversal
- QuickTime metadata extraction
- vendor container metadata extraction
- stable, lossless metadata access in Go
- pure-Go operation without shelling out to exiftool at runtime

## What `videometa` Is Not For

- EXIF/TIFF payload parsing
- XMP/RDF payload parsing
- IPTC-IIM payload parsing
- image-metadata completeness in general

If a video container happens to carry embedded image metadata, that is outside this package's scope by design.

## Why This Scope

The repo originally explored a broader “extract everything exiftool can see” direction. That turned out to be the wrong long-term package boundary. A durable `videometa` should own:

1. video container navigation
2. video-native metadata families
3. video-specific API choices around namespaces, repeated tags, and streaming

and it should not become a second image-metadata library.

## Current Validation Strategy

- exiftool remains the correctness oracle
- real video fixtures back public support claims
- synthetic fixtures remain regression coverage only

## Current Market Fit

The best fit remains projects that:

- catalog or organize video libraries
- need metadata without a Perl dependency
- care about streaming, cross-compilation, and pure-Go deployability
- need vendor video metadata that generic MP4 parsers do not expose
