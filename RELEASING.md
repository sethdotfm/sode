# Releasing

How a change becomes an image people can pull. Adapted to Sode: one maintainer, one image, hand-written changelog, no staging environment. The rules marked **MUST** are enforced by CI, not by habit.

## Versions

SemVer: tag `vX.Y.Z`, image tag `X.Y.Z` (no `v`). The `Stage` word ("Beta") is branding, not part of the number.

- **MAJOR**: breaks something a user relies on: `config.yaml` format or meaning, a removed setting or feature, the data files (`history.jsonl`, `sent.jsonl`, `tokens.json`), the WebSocket or health API.
- **MINOR**: a new feature, compatible with the old config and data.
- **PATCH**: fixes only.
- **Release candidate**: `vX.Y.Z-rc.N`, N from 1, a number is never reused.

The version is written in three places, which `TestVersionNoted` keeps in step: `internal/app/version.go` (`Version`), the `**Version:**` line in README.md, and the `## Beta X.Y.Z` heading in CHANGELOG.md. A candidate and its release are the same commit, so those carry the final number (no "(unreleased)") before the first `-rc.1` is tagged.

## Channels

| Image tag | What it is | Moves | Pushed by |
| --- | --- | --- | --- |
| `latest` | the newest release | yes, forward only | `promote` |
| `X.Y.Z` | a release | never | `promote` |
| `X.Y`, `X` | the newest release of that line | yes, forward only within the line | `promote` |
| `rc` | the newest candidate | yes | `rc` |
| `X.Y.Z-rc.N` | one candidate | never | `rc` |
| `edge` | the latest commit on `main`, built by CI | yes | `ci` |
| `sha-<commit>` | one commit on `main` | never | `ci` |

Stable users (and anything with automatic updates) follow `latest` or `X.Y`. Frontier users follow `rc` (candidates, signed) or `edge` (anything merged, unsigned, may be broken). `latest`, `X.Y.Z`, `X.Y` and `X` MUST come only from `promote.yml`.

## Cutting a release

1. **Prepare on a branch, merge to `main`.** Bump `Version`, the README line, and give CHANGELOG.md its `## Beta X.Y.Z` section (drop "(unreleased)"). `go test ./internal/app -run TestVersionNoted` checks them; with `SODE_RELEASE_TAG=vX.Y.Z-rc.1` it checks the tag too.
2. **Tag the candidate on `main`:** `git tag vX.Y.Z-rc.1 && git push origin vX.Y.Z-rc.1`. `rc.yml` runs the Go and Node tests, builds the multi-arch image, scans it (no fixable CRITICAL), attests provenance and, last, signs it with cosign (GitHub OIDC, no key). It opens a GitHub pre-release.
3. **Test it** (`ghcr.io/sethdotfm/sode:rc`), against a simulator and then your router. A problem: fix on `main`, bump nothing (the version is already final), tag `-rc.2`.
4. **Promote:** Actions > promote > Run workflow (from `main`) with the rc tag. Run it once with *dry run* ticked first: it checks everything and changes nothing. Promotion does not rebuild: it verifies the candidate's signature (made by `rc.yml`, for that tag, at that commit), tags the git commit `vX.Y.Z`, gives the same image digest its release tags, signs it as a release and publishes the GitHub release from the CHANGELOG section.

An rc and the release promoted from it are one image, so the binary says `Beta X.Y.Z (a1b2c3d)` either way. The commit tells you the build; whether a `vX.Y.Z` tag exists on it tells you it was promoted.

## Hotfix for an older line

Branch `release/X.Y` from `vX.Y.Z`, fix there (and on `main`), bump to `X.Y.(Z+1)`, tag `vX.Y.(Z+1)-rc.1` on that branch, promote from `main` as usual. `promote` never moves `latest`, `X.Y` or `X` backward, so a 1.4.3 cut after 2.0.0 takes only `1.4.3` and `1.4`.

## Bad release

Never delete or overwrite `X.Y.Z`. Fix forward with a patch, say so at the top of the bad release's notes, and promote the fix. To put `latest` back at once, an admin runs `docker buildx imagetools create -t ghcr.io/sethdotfm/sode:latest ghcr.io/sethdotfm/sode@sha256:<previous digest>` and notes it in an issue.

## Verifying an image

```sh
cosign verify ghcr.io/sethdotfm/sode@sha256:<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/sethdotfm/sode/\.github/workflows/promote\.yml@refs/heads/main$'
```

A release digest passes with `promote.yml`; a candidate only with `rc.yml@refs/tags/vX.Y.Z-rc.N`; an `edge` or `sha-` image is not signed. For a router you can't afford to surprise, pin `image: ghcr.io/sethdotfm/sode@sha256:...` (from the release notes) and update by hand.

## One-time repository setup

Not in the workflows; set in GitHub:

- Settings > Rules: a ruleset for tags `v*`: block deletion and force-update; creation only by repository admins (the promote workflow pushes `vX.Y.Z` with `GITHUB_TOKEN`, so allow the `github-actions` app as a bypass actor).
- Settings > Actions > General: default workflow permissions read-only.
- Settings > Environments: create `release` (promote runs in it). Adding yourself as required reviewer makes promotion a second deliberate click; leave "prevent self-review" off, you are the only maintainer.
- Branch protection on `main`: require the `ci` checks. Merge commits stay allowed (the history is kept as written).
