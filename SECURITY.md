DISCLOSURE: This was written by an LLM.

# Security policy

Sode drives a broadcast router, so please report a vulnerability privately and not in a public issue.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. The report reaches the maintainer as a private advisory, where the fix and the published advisory are worked out with you. There is no email address for reports.

Please include the Sode version, which Settings shows, how to reproduce the problem, and what an attacker gains.

## Supported versions

Fixes go into the latest release line, the highest `X.Y` in the [releases](https://github.com/sethdotfm/sode/releases). Until a line has a release, its newest rc image gets the fix. Older lines and `edge` or `sha-` images get no fixes; update to the latest release.

## Scope

In scope:

- the Sode server
- the web UI
- the WebSocket and `/api/v1`
- the published images on `ghcr.io/sethdotfm/sode`

Out of scope:

- your TLS proxy, which the README asks you to put in front of Sode
- the Ultrix router and the SW-P-08 protocol itself
- deployments that skip the documented proxy setup
