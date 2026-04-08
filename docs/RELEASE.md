# Release Process

`videometa` is release-on-merge.

## Source of truth

- `VERSION` contains the exact semver tag that the next merge to `main` will publish.
- `docs/releases/<VERSION>.md` is the reviewed GitHub Release body.

## PR requirements

Every release-producing PR must:

1. bump `VERSION`
2. add or update `docs/releases/<VERSION>.md`
3. pass:
   - `release-guard`
   - `hosted-verify`

## Validated fixture corpus

Hosted CI hard-gates the full validated corpus.

- `testdata/IMG_5179.MOV`, `testdata/google.mp4`, and `testdata/sony_a6700.mp4` are committed validated fixtures.
- `testdata/gopro_action.mp4`, `testdata/dji_inspire3_car_4k120_rec709.mov`, and `testdata/dji_ronin4d_4k_prores4444_25fps.mov` are bootstrap-downloadable validated fixtures restored by `scripts/check-local-fixtures.sh`.
- There is no self-hosted runner requirement and no developer-machine background service in the release path.

## Branch protection

After the CI jobs exist on `main`, apply the required checks:

```bash
./scripts/configure-branch-protection.sh
```

That makes `main` require:

- pull-request-based merges
- up-to-date required checks
- `release-guard`
- `hosted-verify`
- conversation resolution

## Release publication

On every push to `main`, `.github/workflows/release.yml`:

1. reruns hosted verification on the merge commit
2. restores the bootstrap-downloadable validated fixtures in hosted CI
3. creates an annotated git tag from `VERSION`
4. publishes the GitHub Release using `docs/releases/<VERSION>.md`

If the tag already exists or hosted verification fails, publication must stop.
