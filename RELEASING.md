# Releasing

How a change becomes an image people can pull. One maintainer, one image, a hand-written changelog, no staging environment. This describes what is set up and has run; the last section lists what is not.

## Versions

SemVer: git tag `vX.Y.Z`, image tag `X.Y.Z` (no `v`). "Beta" is branding, not part of the number.

- **MAJOR**: breaks something a user relies on. Examples are the `config.yaml` format or meaning, a removed setting or feature, the data files (`history.jsonl`, `sent.jsonl`, `tokens.json`), the WebSocket or health API.
- **MINOR**: a new feature, compatible with the old config and data.
- **PATCH**: fixes only.
- **Release candidate**: `vX.Y.Z-rc.N`, N from 1. A number is never reused, even if its build failed.

`TestVersionNoted` keeps the version in step in three places: `Version` in `internal/app/version.go`, the `**Version:**` line in README.md and the `## Beta X.Y.Z` heading in CHANGELOG.md. A candidate and the release promoted from it are the same commit. For this reason, the heading must already be final (no "(unreleased)") before you tag the first `-rc.1`. With `SODE_RELEASE_TAG` set, the test refuses "(unreleased)" for rc tags too.

The Dockerfile builds the binary with the git commit (`-X ...app.Commit`), so Settings shows `Beta 1.5.0 (a1b2c3d)`. An rc and its release are one image and say the same. The commit tells you the build. If a `vX.Y.Z` tag exists on that commit, the image was promoted.

## Image tags (`ghcr.io/sethdotfm/sode`)

| Tag | What it is | Moves | Pushed by |
| --- | --- | --- | --- |
| `latest` | the newest release | yes, forward only | `promote.yml` |
| `X.Y.Z` | a release | never | `promote.yml` |
| `X.Y`, `X` | newest release of that line | yes, forward only within the line | `promote.yml` |
| `rc` | the newest candidate | yes | `rc.yml` |
| `X.Y.Z-rc.N` | one candidate | never | `rc.yml` |
| `edge` | the latest commit on `main` | yes | `ci.yml` |
| `sha-<40 hex>` | one commit on `main` | never | `ci.yml` |

Automatic updaters should follow `latest` (or `X.Y`); only `promote.yml` moves those. Frontier users follow `rc` (signed candidates) or `edge` (whatever is merged: not signed, and it is possible that it is broken). Images are `linux/amd64` and `linux/arm64`.

## The workflows

- **`ci.yml`**: on every PR and every push to `main` or `integration/**`. It runs the test groups (AGENTS.md "Test groups") as parallel jobs, and `ci-ok <base>` sums them up; branch protection on `main` requires only `ci-ok main`. On `main` it then builds and pushes `edge` and `sha-*`. A newer push cancels an older build still running, so `edge` cannot end up on an older commit. The Docker build does not run tests.
- **`rc.yml`**: on a tag `vX.Y.Z-rc.N`. It does these steps in order:
  1. It checks that the commit is on `main` or a `release/X.Y` branch.
  2. It runs vet and all tests.
  3. It runs `TestVersionNoted` for this tag.
  4. It checks that the image tag does not already exist.
  5. It builds and pushes the image (`X.Y.Z-rc.N` and `rc`).
  6. It runs a Trivy scan (no fixable CRITICAL).
  7. It makes a build-provenance attestation.
  8. It adds the **cosign signature last**, so a signed image has passed everything above.
  9. It makes a GitHub pre-release with the CHANGELOG section.

  Trivy runs from its container image pinned by digest (`ghcr.io/aquasecurity/trivy@sha256:...`), because the trivy-action's installer failed to download the binary on the runners (rc.2 and rc.3). Dependabot does not track the digest: bump it by hand now and then. Dependabot does not track `go run` pins either, so bump the staticcheck pin in ci.yml's lint job (`go run honnef.co/go/tools/cmd/staticcheck@v0.8.2`) by hand too. 2025.1.1 and 2026.1 fail on Go 1.27.3 with "export data version 5 > max 2". 2026.2.2 (v0.8.2) is the first that works.
- **`promote.yml`**: run by hand from `main`, with the rc tag and a *dry run* box. It never rebuilds. It does these steps in order:
  1. It checks that the tag is an rc on `main` or a release branch.
  2. It runs `TestVersionNoted` again at that commit.
  3. It finds the rc image's digest and verifies its cosign signature (identity `rc.yml` for exactly that tag and commit).
  4. It refuses if `X.Y.Z` already points at another image.
  5. If the run is a dry run, it stops here.
  6. It creates the annotated git tag `vX.Y.Z`.
  7. It gives that digest `X.Y.Z` and the moving tags.
  8. It signs the digest again as a release.
  9. It publishes the GitHub release from the CHANGELOG.

  It only moves `latest`, `X.Y` and `X` where the new release is the highest of that line. It is safe to run it again after a partial failure. It runs in the `release` environment and refuses to run from any branch but `main`.
- **Dependabot** (`.github/dependabot.yml`): weekly PRs for Actions (they are pinned by commit SHA with the version in a comment), Go modules and the Docker base image. It skips releases younger than 3 days (`cooldown`). Nothing merges by itself.

## Cutting a release

1. **Prepare on a branch and merge to `main`**: bump `Version`, the README line, and give CHANGELOG.md its final `## Beta X.Y.Z` section (no "(unreleased)"). Append the bullets from each `changelog.d/*.md` fragment under that heading, then `git rm` every fragment except README.md; with `SODE_RELEASE_TAG` set, `TestVersionNoted` fails while any remain. `SODE_RELEASE_TAG=vX.Y.Z-rc.1 go test ./internal/app -run TestVersionNoted` checks it before you tag.
2. **Tag the candidate on `main`**: `git tag -a vX.Y.Z-rc.1 -m "..." && git push origin vX.Y.Z-rc.1`. Watch the `rc` run. If it fails, the failed number is spent: fix on `main` and tag `-rc.2`. A candidate that failed before its last step is never signed, so it cannot be promoted.
3. **Test it**: `docker pull ghcr.io/sethdotfm/sode:rc`, against the simulator and then your router.
4. **Promote**: Actions > promote > Run workflow, branch `main`, the rc tag. Do it one time with *dry run* ticked, then again unticked.

History of the first run (1.5.0): the version check refused rc.1 (the heading still said "unreleased"). rc.2 and rc.3 failed in the Trivy installer. rc.4 passed and is the first signed candidate. rc.2 and rc.3 images remain in the registry, unsigned. `promote.yml` has not yet been run for real.

## Hotfix for an older line

Branch `release/X.Y` from `vX.Y.Z`, fix there (and on `main`), bump to `X.Y.(Z+1)`, tag `vX.Y.(Z+1)-rc.1` on that branch, promote from `main` as usual. Promotion never moves `latest`, `X.Y` or `X` backward, so a 1.4.3 cut after 2.0.0 gets only `1.4.3` and `1.4`.

## Bad release

Never delete or overwrite `X.Y.Z`. Fix forward with a patch and say so at the top of the bad release's notes. To put `latest` back immediately, an admin runs `docker buildx imagetools create -t ghcr.io/sethdotfm/sode:latest ghcr.io/sethdotfm/sode@sha256:<previous digest>` and notes it in an issue.

## Verifying an image

```sh
cosign verify ghcr.io/sethdotfm/sode@sha256:<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/sethdotfm/sode/\.github/workflows/promote\.yml@refs/heads/main$'
```

A release digest passes with `promote.yml`; a candidate only with `rc.yml@refs/tags/vX.Y.Z-rc.N`; an `edge` or `sha-` image has no signature. For a router you cannot afford to surprise, pin `image: ghcr.io/sethdotfm/sode@sha256:...` (the digest is in the release notes) and update by hand.

## Repository settings (checked 2026-11-14)

| Setting | State |
| --- | --- |
| Default workflow token permissions | read-only |
| Container package visibility | public |
| Actions pinned by commit SHA | yes, all workflows |
| Tag ruleset for `v*` | "Protect release tags": blocks deletion and non-fast-forward; creating tags is allowed, so promote can push `vX.Y.Z` |
| `release` environment | required reviewer: the repo owner; each promote waits for approval |
| Branch protection on `main` | `ci-ok main` required, branch must be up to date (strict), enforced for admins |
| Merge settings | auto-merge and the Update branch button on; head branches deleted on merge; merge commits, squash and rebase allowed |
| Dependabot | version updates (actions, gomod, docker), vulnerability alerts and security update PRs |
| Private vulnerability reporting | on (SECURITY.md points reporters at it) |
| CodeQL | default setup for actions, go and javascript-typescript; findings land in the Security tab, not a required check |

With strict on, a PR behind main must be updated before it merges. Turn on auto-merge and use Update branch instead of waiting.
