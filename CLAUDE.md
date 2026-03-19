# videometa

Go package for reading metadata from video files. Companion to [bep/imagemeta](https://github.com/bep/imagemeta).

## Status

**v0.1.0.** All decoders complete: ISOBMFF, EXIF, XMP, IPTC, QuickTime native, Pentax MakerNotes, Sony XAVC (UUID-PROF, USMT/MTDT, NRTM XML), Apple MOV (mdta locales, wave/frma). Zero golden gaps across all test files.

See `INIT.md` for project history. See `docs/` for requirements, architecture, and task plan. See `README.md` for usage.

## Routing Table

| When modifying... | Read first | Also consult |
|---|---|---|
| `videometa.go` | `docs/REQUIREMENTS.md` §2 (API) | `docs/ARCHITECTURE.md` §1 (Data Flow) |
| `videodecoder_mp4.go` | `docs/ARCHITECTURE.md` §3 (Box Parser) | `docs/REQUIREMENTS.md` §3 (ISOBMFF, QuickTime) |
| `metadecoder_exif*.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (EXIF) |
| `metadecoder_xmp.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (XMP) |
| `metadecoder_iptc*.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (IPTC) |
| `metadecoder_quicktime*.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (QuickTime) |
| `metadecoder_makernotes_pentax.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (EXIF-07..09) |
| `metadecoder_sony_nrtm.go` | `docs/ARCHITECTURE.md` §4 (Decoders) | `docs/REQUIREMENTS.md` §3 (QuickTime) |
| `io.go` | `docs/ARCHITECTURE.md` §5 (streamReader) | |
| `helpers.go` | `docs/ARCHITECTURE.md` §6 (Error Handling) | |
| `gen/`, `testdata/` | `docs/ARCHITECTURE.md` §7 (Testing) | `docs/REQUIREMENTS.md` §5 (Test Corpus) |
| `.github/workflows/` | `docs/ARCHITECTURE.md` §7 (Testing) | `docs/REQUIREMENTS.md` §4 (REQ-NF-10) |

| Working on... | Requirements | Architecture |
|---|---|---|
| Public API | `docs/REQUIREMENTS.md` §2 | `docs/ARCHITECTURE.md` §1, §2 |
| ISOBMFF box parsing | `docs/REQUIREMENTS.md` §3 (BOX) | `docs/ARCHITECTURE.md` §3 |
| Metadata decoding | `docs/REQUIREMENTS.md` §3 | `docs/ARCHITECTURE.md` §4 |
| I/O layer | `docs/REQUIREMENTS.md` §4 (NF-01, NF-02) | `docs/ARCHITECTURE.md` §5 |
| Error handling | `docs/REQUIREMENTS.md` §4 (NF-06) | `docs/ARCHITECTURE.md` §6 |
| Testing | `docs/REQUIREMENTS.md` §4, §5 | `docs/ARCHITECTURE.md` §7 |

Planned work: see `docs/TASKS.md`.

## Eng Philosophy

- Prefer large, long-term solutions over quick fixes. Do big re-architectures early, not late.
- Never settle for "good enough for now."
- Never treat current implementation as a reason to avoid change.
- Modules and packages can be rethought at a whim if a better design appears. No code is sacred.
- App hasn't been launched yet. No backwards compatibility. Ensure after refactoring the code doesn't show any signs of the old architecture.

### Ownership — you own this repo

- Never leave the repo in a broken state (build fails, tests fail, lint errors)
- Never call issues "pre-existing" — you find it, you fix it
- If you touch a file, leave it better than you found it
- If something is broken, fix it — don't work around it

## Project-Specific Rules

### exiftool Is the Source of Truth

Every tag name, every value conversion must match `exiftool -n -json` output. When in doubt, exiftool wins. Study the Perl source (`QuickTime.pm`, `Exif.pm`, `XMP.pm`) for edge cases and value conversion logic.

### Tag Name Exactness

Tag names are part of the API contract. They must match exiftool output character-for-character. This is tested via golden files, not aspirational.

### Panic-Based Control Flow (Internal Only)

Internal decoders use `panic(errStop)` on EOF/error, recovered at the `Decode()` boundary. This is intentional (imagemeta pattern), not a bug. **Never let these panics escape the public API.**

### Streaming Constraint

Never buffer an entire file or even an entire box into memory. `mdat` can be gigabytes. Seek or discard, never read-and-hold. Use streamReader's convenience methods for all binary I/O.

### No CGo Invariant

`go build` must work with `CGO_ENABLED=0`. No exceptions. This enables cross-compilation and static binaries.

### Golden File Workflow

`go generate ./gen` runs exiftool on test videos, produces JSON in `testdata/`. Tests compare videometa output against committed JSON. CI re-runs exiftool to catch drift. This is the primary correctness mechanism.

### Fuzz Testing Mandate

Every decoder path gets a fuzz target. The rule: no panics, no allocations > 10MB, `InvalidFormatError` for anything malformed. Seed corpus from real test videos.

### Binary Format Parsing

ISOBMFF is always big-endian. EXIF can be either endianness. Always use streamReader's byte-order-aware methods. Never use `encoding/binary` directly. Validate all sizes before allocating (fuzz defense).

### Test Corpus Management

Small test videos (< 50 KB) are committed to git. Large real-world test videos are **gitignored but live on disk** — they are the user's data and must never be deleted, nor may their golden JSON or test functions be removed.

**Large gitignored test files (DO NOT DELETE):**

| File | Size | Provenance | Golden JSON (committed) |
|------|------|-----------|------------------------|
| `testdata/apple.mov` | 110 MB | iPhone 15 Pro, HEVC | `apple.mov.exiftool.json` |
| `testdata/sony_a6700.mp4` | 67 MB | Sony A6700, XAVC | `sony_a6700.mp4.exiftool.json` |

These files have conditional-skip golden tests (`TestGoldenAppleMOV`, `TestGoldenSonyA6700`) that run when the file is present and skip gracefully in CI. When working in a worktree, copy them from the main repo (see Step 2 in Dev Process).

### Decoder Approach

Implement from specs. Use exiftool's Perl logic as reference for edge cases and value conversions. Can reuse imagemeta code we authored. Do not depend on imagemeta as a module.

## Coding Conventions

### General

- Write comments explaining **why**, not **what**
- Functions do one thing
- Accept interfaces, return structs
- No package-level mutable state
- No magic numbers — use named constants near their usage
- Always use named fields in struct literals — positional initialization breaks silently when fields are added
- Unexported by default. Export only what other packages need. The exported API is a contract.

### Naming

- **Names carry the semantics the type can't.** `count int` is useless. `pendingBoxCount int` is self-documenting. A name should let you understand usage without reading the definition.
- **Boolean names state the true condition.** `isFullBox`, `hasExtendedSize`, `canSeek` — never negated names like `notDone` (double negatives in `if !notDone` are unreadable).
- **Package names are single lowercase words.** No underscores, no `util`, no `common`, no `helpers`. If you can't name the package, the abstraction is wrong.

### Error Handling

- **Wrap with `fmt.Errorf("verb noun: %w", err)`** — the message reads as a chain: `"decode exif: read ifd: unexpected EOF"`. Verb-noun, not "failed to" or "error while".
- **Errors cross exactly one boundary before being wrapped.** Don't double-wrap.
- **Sentinel errors are for callers that branch on them.** `InvalidFormatError`, `ErrStopWalking` — these exist because callers check them. If no caller checks, a formatted string is fine.
- **Never swallow errors.** If you handle an error (skip, partial result), surface it via `Warnf` callback. If you can't handle it, return it.
- **Panics are internal flow control only.** `panic(errStop)` in decoders, recovered at the `Decode()` boundary. Never panic on external input. Never let a panic escape the public API.
- **Partial failure is a first-class concept.** EXIF decode error doesn't prevent XMP extraction. Collect what you can, report what failed.

### Resource Lifecycle

- **`defer` for cleanup, but verify the close.** For writes: check the error from `Close()`. Pattern: `defer func() { closeErr := f.Close(); if err == nil { err = closeErr } }()`.
- **Streams over buffers.** Never read an entire file or box into memory unless the size is bounded and small (< 1 MB). Use `io.Reader`/`io.Writer` pipelines. `mdat` can be gigabytes — seek past it.

### Dependencies

- Zero runtime dependencies (IPTC charset decoding is done with stdlib)
- Test dependencies: `frankban/quicktest`, `google/go-cmp` (following imagemeta)
- Evaluate every new dependency for maintenance health, transitive deps, and whether the functionality justifies the coupling
- Prefer stdlib over third-party when the stdlib solution is reasonable

### Test Style

- **All assertions use quicktest** (`github.com/frankban/quicktest`), following imagemeta's pattern.
- **Requirement traceability**: Every test that validates a spec requirement MUST have a `// Validates: REQ-*` comment on the line immediately before the `func Test...` declaration. Multiple requirements use comma separation: `// Validates: REQ-BOX-01, REQ-BOX-02`. For table-driven subtests, place the comment on the subtest case struct. This enables `grep -r "Validates:"` to produce a full traceability matrix.
- Table-driven tests where appropriate, with specific assertions (check values, not just "no error")

### Test Strategy

- **Test the contract, not the implementation.** Tests should break when behavior changes, not when you refactor internals. Test through exported APIs.
- **Golden files are the primary validation mechanism.** Compare videometa output against committed exiftool JSON. Update with `go generate ./gen`.
- **Fuzz every decoder path.** Seed corpus from real test videos. Must not panic, must not allocate > 10MB.
- **Benchmarks are required.** Per-source, all-sources, per-file-type. Target: < 500μs for typical smartphone MP4.

## Dev Process

Work is done in increments. Do not ask permission, do not skip any step.

### Step 1: Claim work

1. Read `docs/TASKS.md` for the next milestone.
2. Read the governing docs (see Routing Table above).
3. Evaluate the codebase to determine if any foundational improvements are needed before starting.

### Step 2: Set up worktree

1. Create a worktree using tool.
2. Create a branch with the naming convention: `<type>/<task-name>` (e.g., `feat/isobmff-parser`). Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.
3. **Copy gitignored test files into the worktree.** Large test videos (`apple.mov`, `sony_a6700.mp4`, etc.) are gitignored and won't appear in worktrees. Copy or symlink them from the main repo:
   ```
   for f in testdata/apple.mov testdata/sony_a6700.mp4; do
     [ -f "/Users/tonimelisma/Development/videometa/$f" ] && \
       cp "/Users/tonimelisma/Development/videometa/$f" "<worktree>/$f"
   done
   ```
   **Never delete gitignored test files or their golden JSON. They are the user's data.**
4. All changes go through PRs.

### Step 3: Develop with TDD

All development follows strict red/green/refactor TDD. Mandatory regression tests for every bug fix.

### Step 4: Update docs

Mandatory, not optional:
- **Architecture/requirements**: update if behavior changed or new constraints discovered.
- **Task status**: mark tasks complete in `docs/TASKS.md`.
- **Traceability matrix**: keep `docs/REQUIREMENTS.md` §6 current.

### Step 5: Self-verify

Re-read the governing design doc. Produce a compliance report listing each spec item, whether it was implemented in full, partially, or not at all, and how it was implemented.

### Step 6: Code review checklist

Self-review every change against coding standards proceeding to the Definition of Done.

### Step 7: Definition of Done

After each increment, run through this entire checklist. If something fails, fix and re-run from the top. **When complete, present this checklist to the human with pass/fail status for each item.**

1. [ ] **Format**: `gofumpt -w . && goimports -local github.com/tonimelisma/videometa -w .`
2. [ ] **Lint**: `golangci-lint run`
3. [ ] **Build**: `CGO_ENABLED=0 go build ./...`
4. [ ] **Unit tests**: `go test -race -coverprofile=/tmp/cover.out ./...`
5. [ ] **Coverage**: `go tool cover -func=/tmp/cover.out | grep total`
6. [ ] **Golden file validation**: `go generate ./gen` produces no diff
7. [ ] **Docs updated**: CLAUDE.md, docs/ as needed
8. [ ] **Push and CI green**: Push branch, open PR with `gh pr create`, then enable auto-merge with `gh pr merge --auto --squash --delete-branch`. Monitor with `gh pr checks <pr_number> --watch`.
9. [ ] **Cleanup**: From the root repo (not worktree), remove the worktree after merge. Force-delete the local branch with `git branch -D` (squash merges create a new commit on main). Prune and pull:
    ```
    cd /Users/tonimelisma/Development/videometa
    git worktree remove <worktree-path>
    git branch -D <branch-name>
    git fetch --prune origin
    git checkout main && git pull --ff-only origin main
    ```
    **NEVER delete other worktrees or branches — even if they appear stale.** Report them to the human instead.
10. [ ] **Increment report**: Present to the human:
    - **What you changed**: What files did you change, why and how
    - **Plan deviations**: For every deviation from the approved plan
    - **Top-up recommendations**: Any remaining codebase improvements you'd make
    - **Unfixed items**: Anything you were unable to address in this increment
