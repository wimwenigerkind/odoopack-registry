# Releasing

A release is a git tag `vX.Y.Z` on `main`. Nothing else publishes a version:
neither a branch nor a pre-release tag (`v1.2.0-rc.1` does not trigger a
release).

## Procedure

1. Bump `version` in `charts/odoopack/Chart.yaml` and merge it to `main`. Every
   release changes the chart's `appVersion`, so every release needs a new chart
   version; the workflow refuses to overwrite one that is already published.
2. Tag the merged commit and push the tag:

   ```sh
   git switch main && git pull
   git tag v1.2.3
   git push origin v1.2.3
   ```

3. Watch the CI run for the tag. When it is green, the release exists on the
   repository's Releases page.

If the run fails before the images are pushed (a test, or a chart version that
already exists), fix it on `main`, then move the tag: `git tag -d v1.2.3`,
`git push origin :v1.2.3`, tag again and push.

## What a tag builds

CI runs the same checks as on `main` (tests, integration tests, lint,
frontend, Helm, image scan), then:

| Artifact | Tags for `v1.2.3` | Tags for `v0.5.0` |
|----------|-------------------|-------------------|
| `ghcr.io/wimwenigerkind/odoopack-registry-backend` | `1.2.3`, `1.2`, `1`, `latest` | `0.5.0`, `0.5`, `latest` |
| `ghcr.io/wimwenigerkind/odoopack-registry-frontend` | `1.2.3`, `1.2`, `1`, `latest` | `0.5.0`, `0.5`, `latest` |
| `oci://ghcr.io/wimwenigerkind/charts/odoopack` | version from `Chart.yaml`, `appVersion` `1.2.3` | version from `Chart.yaml`, `appVersion` `0.5.0` |

A `0` tag is never pushed: below 1.0.0 every minor release may break, so a tag
spanning all of them would be no safer than `latest`.

The backend binaries carry the version: `web` and `worker` log it when they
start, and the web's `/healthz` answers with it
(`{"status":"ok","version":"1.2.3"}`).

Last, the workflow creates a GitHub release for the tag with notes generated
from the pull requests merged since the previous tag.

## Between releases

Every push to `main` pushes both images as `main-<sha>` only, and the binaries
report `main-<sha>` as their version. `latest` moves only with a release, so it
always names a tested, tagged version. The chart is published only by a
release: a chart built from `main` would default to an `appVersion` whose
images may not exist.

Tagging a patch for an older line after a newer release (`v1.1.4` after
`v1.2.0`) moves `latest` back to it; release from `main` only.
