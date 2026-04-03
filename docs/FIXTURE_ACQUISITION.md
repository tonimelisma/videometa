# Fixture Acquisition

This document tracks the local real-video fixtures already in use plus the remaining fixture gaps and how to obtain them.

The rules are:

- Real-video validation fixtures must be original `.mp4` or `.mov` files.
- Local-only large fixtures stay in `testdata/` and are gitignored.
- Synthetic or generated media can still exist for parser regression tests, but they do not count as validation evidence.
- Do not transcode, rewrap, or export through photo/video apps before copying the file into `testdata/`. We want the original metadata-bearing container.

## Quick Start

Run the bootstrap script to download every fixture that currently has a verified official direct-download URL:

```bash
./scripts/bootstrap-fixtures.sh
```

The script is intentionally full-service, but it now downloads smaller fixtures first so the commonly used validation assets come back quickly instead of blocking on the multi-hundred-megabyte DJI samples.

Today that means three fixtures:

- `testdata/dji_ronin4d_4k_prores4444_25fps.mov`
- `testdata/dji_inspire3_car_4k120_rec709.mov`
- `testdata/gopro_action.mp4`

Together they require about 1.71 GiB of disk space.

The script is intentionally manifest-driven:

- `scripts/fixture_bootstrap.tsv` is the source of truth for scriptable fixtures.
- If we find another verified direct-download URL later, add one row to the TSV and the script will pick it up automatically.

## Acquired Local Fixtures

These real files are already part of the local validation corpus:

| Local filename | Why it matters | Acquisition mode | Current source |
|---|---|---|---|
| `testdata/google.mp4` | Android phone clip with real smartphone metadata and GPS behavior | Manual local file already present | Pixel 9 Pro original recording |
| `testdata/gopro_action.mp4` | GoPro action-camera clip with real GoPro metadata | Scriptable now | [GoPro share page](https://gopro.com/v/8GodrO3G8bNK4) |
| `testdata/apple.mov` | Apple / iPhone MOV fixture | Manual local file already present | iPhone 15 Pro original recording |
| `testdata/sony_a6700.mp4` | Sony professional-camera fixture | Manual local file already present | Sony A6700 original recording |

## Remaining Fixture Gaps

| Local filename | Why we need it | Acquisition mode | Current source |
|---|---|---|---|
| `testdata/dji_ronin4d_4k_prores4444_25fps.mov` | Professional camera fixture beyond the existing Sony A6700 sample | Scriptable now | [DJI Ronin 4D Samples](https://www.dji.com/ronin-4d/samples) |
| `testdata/dji_inspire3_car_4k120_rec709.mov` | DJI drone fixture | Scriptable now | [DJI Inspire 3 Samples](https://www.dji.com/inspire-3/samples) |
| `testdata/panasonic_pro_camera.mov` | Panasonic/LUMIX professional camera fixture | Manual only | [LUMIX Cinema](https://shop.panasonic.com/pages/lumix-cinema), [GH5 Video Gallery](https://www.panasonic.com/global/consumer/lumix-index/gh/gh5_video.html), [GH5S Video Gallery](https://www.panasonic.com/global/consumer/lumix-index/gh/gh5s_video.html) |
| `testdata/canon_pro_camera.mov` | Canon professional camera fixture | Manual only | [Canon EOS C70](https://www.usa.canon.com/internet/portal/us/home/products/details/cameras/cinema-eos/eos-c70), [Canon EOS R5 C](https://www.usa.canon.com/internet/portal/us/home/products/details/cameras/cinema-eos/eos-r5-c) |
| `testdata/exif_uuid_video.mov` | Real video carrying EXIF in a BMFF `uuid` box | Manual only | No verified public original-file download URL currently tracked |
| `testdata/exif_meta_iloc_video.mov` | Real video carrying EXIF via `meta` item extraction (`iloc` family) | Manual only | No verified public original-file download URL currently tracked |
| `testdata/xmp_uuid_video.mov` | Real video carrying XMP in a BMFF `uuid` box | Manual only | No verified public original-file download URL currently tracked |
| `testdata/xmp_meta_iloc_video.mov` | Real video carrying XMP via `meta` item extraction (`iloc` family) | Manual only | No verified public original-file download URL currently tracked |
| `testdata/iptc_applicationnotes_video.mov` | Real video carrying IPTC-IIM via EXIF `ApplicationNotes` | Manual only | No verified public original-file download URL currently tracked |
| `testdata/box64_extended_size.mp4` | Parser/conformance fixture for 64-bit box sizes | Crafted locally, not downloaded | Create a tiny synthetic fixture; this does not need to be camera footage |

## Scriptable Fixtures

### DJI Ronin 4D

- Local path: `testdata/dji_ronin4d_4k_prores4444_25fps.mov`
- Official page: [DJI Ronin 4D Samples](https://www.dji.com/ronin-4d/samples)
- Verified direct asset URL: embedded in `scripts/fixture_bootstrap.tsv`
- Why it matters: gives us a second professional-camera MOV sample from a different vendor family.

### DJI Inspire 3

- Local path: `testdata/dji_inspire3_car_4k120_rec709.mov`
- Official page: [DJI Inspire 3 Samples](https://www.dji.com/inspire-3/samples)
- Verified direct asset URL: embedded in `scripts/fixture_bootstrap.tsv`
- Why it matters: gives us a DJI drone video sample. This official sample is MOV, not MP4, which is acceptable for the repo's MOV/MP4 validation corpus.

### GoPro HERO12 shared clip

- Local path: `testdata/gopro_action.mp4`
- Public page: [GoPro share page](https://gopro.com/v/8GodrO3G8bNK4)
- Bootstrap behavior: the script parses the public share page URL, calls GoPro's public media APIs with the same `Accept` headers used by the site, and resolves a fresh signed `source` download URL on demand.
- Why it matters: gives us a real GoPro MP4 with intact GoPro metadata tracks and GoPro `udta`/GPMF vendor metadata instead of relying on an expiring signed CDN URL.

## Already Acquired Manually

### Google Pixel 9 Pro clip

- Local path: `testdata/google.mp4`
- Provenance: original Pixel 9 Pro recording copied locally by the user.
- Why it matters: gives us a real Android phone clip with GPS-bearing smartphone metadata and Android `mdta` keys.

## Manual-Only Fixtures

These still need human involvement either because there is no verified public direct-download URL, or because the right fixture is best obtained from an original device file.

### Panasonic / LUMIX professional camera clip

Target path: `testdata/panasonic_pro_camera.mov`

Recommended acquisition:

1. Start from an original LUMIX `.mov` or `.mp4` camera file.
2. Prefer a short clip copied directly from the camera card.
3. Save it as `testdata/panasonic_pro_camera.mov`.

Source pages:

- [LUMIX Cinema](https://shop.panasonic.com/pages/lumix-cinema)
- [GH5 Video Gallery](https://www.panasonic.com/global/consumer/lumix-index/gh/gh5_video.html)
- [GH5S Video Gallery](https://www.panasonic.com/global/consumer/lumix-index/gh/gh5s_video.html)

Public-download status:

- No verified direct-download original asset URL is currently tracked from Panasonic's public pages.

### Canon professional camera clip

Target path: `testdata/canon_pro_camera.mov`

Recommended acquisition:

1. Start from an original Canon Cinema EOS or hybrid-camera video file.
2. Prefer a short clip copied directly from camera media.
3. Save it as `testdata/canon_pro_camera.mov`.

Source pages:

- [Canon EOS C70](https://www.usa.canon.com/internet/portal/us/home/products/details/cameras/cinema-eos/eos-c70)
- [Canon EOS R5 C](https://www.usa.canon.com/internet/portal/us/home/products/details/cameras/cinema-eos/eos-r5-c)

Public-download status:

- No verified direct-download original asset URL is currently tracked from Canon's public pages.

### Rare embedded-route validation fixtures

Targets:

- `testdata/exif_uuid_video.mov`
- `testdata/exif_meta_iloc_video.mov`
- `testdata/xmp_uuid_video.mov`
- `testdata/xmp_meta_iloc_video.mov`
- `testdata/iptc_applicationnotes_video.mov`

Why they matter:

- These are the route-specific real-video fixtures needed to move currently implemented but not yet validated carrier paths into the validated corpus.

Recommended acquisition:

1. Prefer real original video files from known devices or workflows.
2. Inspect candidates with `exiftool -n -json -g` and, when duplicate-heavy vendor metadata is involved, `exiftool -a -n -G0 -S`.
3. Keep only files that actually exercise the route named in the filename.
4. Save each file under the matching target name above.

Public-download status:

- No verified public original-file download URLs are currently tracked for these rare routes.

## After Downloading or Copying a Fixture

1. Place the original file at the exact target path in `testdata/`.
2. Confirm it remains gitignored with `git status --short`.
3. Inspect the file with:

   ```bash
   exiftool -n -json -g testdata/<fixture-name>
   ```

4. If we decide the fixture should participate in the validated corpus, generate or refresh the local grouped and ordered goldens with:

   ```bash
   go generate ./gen
   ```

5. Add or update the corresponding conditional-skip golden test.

## Extending the Bootstrap Script

When you find a new official direct-download URL:

1. Add a row to `scripts/fixture_bootstrap.tsv`.
2. Add the local target path to `.gitignore`.
3. Keep the target filename stable and descriptive.
4. Prefer original camera files, not re-encodes or social exports.

The script intentionally needs no arguments. It downloads every verified fixture in the manifest.
