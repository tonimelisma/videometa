# videometa

Go package for reading metadata from video files. Companion to [bep/imagemeta](https://github.com/bep/imagemeta).

## Status

**v0.1.0.** Implemented decoders: ISOBMFF, EXIF, XMP, IPTC, QuickTime native, `meta/iloc` EXIF/XMP item extraction, Apple/Canon/Sony EXIF MakerNotes, Pentax `TAGS`, Sony XAVC (UUID-PROF, USMT/MTDT, NRTM XML), Apple MOV (mdta locales, wave/frma). Real-file golden coverage is maintained for the committed fixtures; synthetic end-to-end tests cover `meta/iloc` and EXIF MakerNotes paths.

See [CLAUDE.md](/Users/tonimelisma/Development/videometa-integrity/CLAUDE.md) for the full contributor instructions and routing table. See `docs/` for requirements, architecture, and task status.
