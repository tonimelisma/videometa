# videometa

Go package for reading metadata from video files. Companion to [bep/imagemeta](https://github.com/bep/imagemeta) in spirit, but intentionally focused on video-native/container-native metadata only.

## Status

The current release version is recorded in `VERSION`. Implemented decoders: ISOBMFF, QuickTime native, vendor metadata families (`Pentax/moov/udta/TAGS`, Sony UUID-PROF, Sony USMT/MTDT, Sony NRTM, GoPro `udta`/GPMF), Apple MOV (`mdta` locales, `wave`/`frma`), and Android `mdta`. The collected API is lossless and namespace-preserving (`Tags` -> `SourceTags` -> `NamespaceTags`). Real-file golden coverage is maintained for committed fixtures plus bootstrap-downloadable validated fixtures; synthetic tests remain regression coverage only and do not count as validated support claims.

**Explicit non-goal:** embedded image metadata payloads. If a video contains EXIF/TIFF, XMP/RDF, or IPTC-IIM payloads, `videometa` does not parse them.

See `INIT.md` for project history. See `docs/` for requirements, architecture, and task plan. See `README.md` for usage.

## Routing Table

| When modifying... | Read first | Also consult |
|---|---|---|
| `videometa.go` | `docs/REQUIREMENTS.md` §2 (API) | `docs/ARCHITECTURE.md` §1 (Data Flow) |
| `videodecoder_mp4.go` | `docs/ARCHITECTURE.md` §3 (Box Parser) | `docs/REQUIREMENTS.md` §3 (ISOBMFF, QuickTime, Vendor) |
| `metadecoder_quicktime*.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (QuickTime, Vendor) |
| `metadecoder_sony_nrtm.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (Vendor) |
| `io.go` | `docs/ARCHITECTURE.md` §5 (streamReader) | |
| `errors.go`, `datetime.go`, `gps.go`, `text.go`, `value.go` | `docs/ARCHITECTURE.md` §4, §6 | `docs/REQUIREMENTS.md` §2, §4 |
| `gen/`, `testdata/` | `docs/ARCHITECTURE.md` §7 (Testing) | `docs/REQUIREMENTS.md` §5 (Test Corpus) |
| `.github/workflows/` | `docs/ARCHITECTURE.md` §7 (Testing) | `docs/REQUIREMENTS.md` §4 (REQ-NF-10) |

| Working on... | Requirements | Architecture |
|---|---|---|
| Public API | `docs/REQUIREMENTS.md` §2 | `docs/ARCHITECTURE.md` §1, §4 |
| ISOBMFF box parsing | `docs/REQUIREMENTS.md` §3 (BOX) | `docs/ARCHITECTURE.md` §3 |
| Metadata decoding | `docs/REQUIREMENTS.md` §3 | `docs/ARCHITECTURE.md` §4 |
| I/O layer | `docs/REQUIREMENTS.md` §4 (NF-01, NF-02) | `docs/ARCHITECTURE.md` §5 |
| Error handling | `docs/REQUIREMENTS.md` §4 (NF-06) | `docs/ARCHITECTURE.md` §6 |
| Testing | `docs/REQUIREMENTS.md` §4, §5 | `docs/ARCHITECTURE.md` §7 |

Planned work: see `docs/TASKS.md`.

## Eng Philosophy

- Prefer large, long-term solutions over quick fixes.
- Never settle for "good enough for now."
- Never treat current implementation as a reason to avoid change.
- Modules and packages can be rethought at a whim if a better design appears. No code is sacred.
- App hasn't been launched yet. No backwards compatibility. Ensure after refactoring the code doesn't show signs of the old architecture.

### Ownership

- Never leave the repo in a broken state
- Never call issues "pre-existing"
- If you touch a file, leave it better than you found it
- If something is broken, fix it — don't work around it

## Project-Specific Rules

### exiftool Is the Source of Truth

Every supported tag name and value conversion must match exiftool output. Grouped `exiftool -n -json` remains the broad parity baseline, and ordered duplicate-preserving exiftool goldens backstop repeated-tag cases. When in doubt, exiftool wins. Study `QuickTime.pm` and the vendor tag implementations exiftool uses for edge cases and value conversion logic.

### Scope Discipline

- `videometa` is for video-native/container-native metadata only.
- Do not add EXIF/TIFF, XMP/RDF, or IPTC-IIM parsing back into the package.
- Do not add container routes whose only purpose is embedded image metadata (`uuid` EXIF/XMP, `XMP_`, `meta/iloc` image payloads, EXIF `ApplicationNotes`).

### Tag Name Exactness

Tag names are part of the API contract. They must match exiftool output character-for-character.

### Panic-Based Control Flow (Internal Only)

Internal decoders use `panic(errStop)` on EOF/error, recovered at the `Decode()` boundary. Never let these panics escape the public API.

### Streaming Constraint

Never buffer an entire file or even an entire box into memory. `mdat` can be gigabytes. Seek or discard, never read-and-hold. Use `streamReader` helpers for binary I/O.

### No CGo Invariant

`go build` must work with `CGO_ENABLED=0`.

### Golden File Workflow

`go generate ./gen` runs exiftool on test videos and produces grouped JSON plus ordered occurrence goldens in `testdata/`. Tests compare videometa output against those committed artifacts. CI reruns exiftool to catch drift. This is the primary correctness mechanism.

### Release Strategy

- `videometa` releases from `main` only.
- Every merged increment must bump `VERSION` and add `docs/releases/<VERSION>.md`.
- Pre-`v1.0.0` semver still has meaning:
  - patch releases (`v0.x.y`) are for bug fixes, parity fixes, internal refactors, and docs/tests with no intentional public contract change
  - minor releases (`v0.(x+1).0`) are for any exported API change, tag-name change, source/namespace change, support-policy change, or new validated metadata family
- No silent merges to `main`: every merge is release-producing.
- Release notes are mandatory and reviewed in the PR, not written after the fact.

### Release CI

- Hosted CI is required for every PR and every release.
- Hosted CI restores the bootstrap-downloadable validated fixtures with `scripts/check-local-fixtures.sh` before running the hard gate.
- Bootstrap-downloadable fixture tests may skip in ordinary local development, but they must fail hard when `VIDEOMETA_REQUIRE_LOCAL_FIXTURES=1`.
- `push` to `main` reruns hosted verification, restores the bootstrap fixtures, then publishes the Git tag and GitHub Release from `VERSION` and `docs/releases/<VERSION>.md`.
- There is no self-hosted runner or developer-machine background service in the release path.

### Fuzz Testing Mandate

Every supported decoder path gets a fuzz target. The rule: no panics, no allocations > 10MB, `InvalidFormatError` for malformed container input.

### Binary Format Parsing

ISOBMFF is always big-endian. Always use `streamReader`'s byte-order-aware methods. Never use `encoding/binary` directly in parser code paths unless you are working on bounded local test helpers or isolated vendor payload helpers and that choice is justified.

### Test Corpus Management

Committed validated fixtures stay in git:

- `testdata/IMG_5179.MOV`
- `testdata/google.mp4`
- `testdata/sony_a6700.mp4`

Bootstrap-downloadable validated fixtures remain gitignored but are restorable via `scripts/check-local-fixtures.sh`:

| File | Size | Provenance | Golden JSON (committed) |
|------|------|-----------|------------------------|
| `testdata/gopro_action.mp4` | 67 MB | GoPro HERO12 Black public shared clip | `gopro_action.mp4.exiftool.json` |
| `testdata/dji_inspire3_car_4k120_rec709.mov` | 561 MB | DJI Inspire 3 sample | `dji_inspire3_car_4k120_rec709.mov.exiftool.json` |
| `testdata/dji_ronin4d_4k_prores4444_25fps.mov` | 1.19 GB | DJI Ronin 4D sample | `dji_ronin4d_4k_prores4444_25fps.mov.exiftool.json` |

These bootstrap fixtures skip only in ordinary local development. Hosted CI restores them into the checkout and runs the same tests with `VIDEOMETA_REQUIRE_LOCAL_FIXTURES=1`, which turns missing validated fixtures into a hard failure. Legacy local user data such as `testdata/apple.mov` may still exist on disk, but it is no longer part of the validated corpus and must not be deleted.

## Coding Conventions

### General

- Write comments explaining why, not what
- Functions do one thing
- Accept interfaces, return structs
- No package-level mutable state
- No magic numbers; use named constants near their usage
- Always use named fields in struct literals
- Unexported by default

### Error Handling

- Wrap with `fmt.Errorf("verb noun: %w", err)`
- Errors cross exactly one boundary before being wrapped
- Never swallow errors; surface handled partial failures via `Warnf`
- Panics are internal flow control only
- Partial failure is first-class for supported routes: one malformed vendor payload should not abort unrelated supported metadata

### Dependencies

- Zero runtime dependencies
- Test dependencies: `frankban/quicktest`, `google/go-cmp`
- Prefer stdlib when reasonable

### Test Style

- All assertions use quicktest
- Every requirement-validating test must carry `// Validates: REQ-*`
- Table-driven where appropriate, with specific assertions

### Test Strategy

- Test the contract, not the implementation
- Golden files are the primary validation mechanism
- Fuzz every supported decoder path
- Benchmarks are required

## Dev Process

Work is done in increments. Do not ask permission, do not skip steps.

### Step 1: Claim work

1. Read `docs/TASKS.md` for the next milestone.
2. Read the governing docs.
3. Evaluate whether foundational improvements are needed before starting.

### Step 2: Set up worktree

1. Create a worktree.
2. Create a branch named `<type>/<task-name>`.
3. Committed validated fixtures are already present in the worktree. If you need the bootstrap-downloadable validated fixtures, run `./scripts/check-local-fixtures.sh`.
4. All changes go through PRs.

### Step 3: Develop with TDD

All development follows strict red/green/refactor TDD. Mandatory regression tests for every bug fix.

### Step 4: Update docs

Mandatory:

- Update architecture/requirements if behavior or constraints changed.
- Update `docs/TASKS.md`.
- Keep the traceability matrix current.
- Bump `VERSION` and add `docs/releases/<VERSION>.md` for every release-producing increment.

### Step 5: Self-verify

Re-read the governing design doc. Produce a compliance report listing each spec item, whether it was implemented in full, partially, or not at all, and how.

### Step 6: Code review checklist

Self-review every change against coding standards before Definition of Done.

### Step 7: Definition of Done

After each increment, run through the entire checklist. If something fails, fix and rerun from the top. When complete, present this checklist to the human with pass/fail status for each item.

1. [ ] Format: `gofumpt -w . && goimports -local github.com/tonimelisma/videometa -w .`
2. [ ] Lint: `golangci-lint run`
3. [ ] Build: `CGO_ENABLED=0 go build ./...`
4. [ ] Unit tests: `go test -race -coverprofile=/tmp/cover.out ./...`
5. [ ] Coverage: `go tool cover -func=/tmp/cover.out | grep total`
6. [ ] Golden file validation: `go generate ./gen` produces no diff
7. [ ] Docs updated: `CLAUDE.md`, `docs/`, `README.md` as needed
8. [ ] Release metadata updated: `VERSION` bumped and `docs/releases/<VERSION>.md` added
9. [ ] Push and CI green: push branch, open PR, enable auto-merge, watch checks
10. [ ] Cleanup: remove worktree, delete local branch, prune, and fast-forward root `main`
11. [ ] Increment report: summarize changes, plan deviations, top-up recommendations, unfixed items
