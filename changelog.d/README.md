# Changelog fragments

A PR with a user-visible change adds `changelog.d/<branch-or-slug>.md` holding one or more `- ` bullets in CHANGELOG.md style.
Never edit CHANGELOG.md, `Version` or the README Version line in a feature PR; a release folds the fragments in (RELEASING.md).
`TestChangelogFragments` checks the format and refuses fragments when `SODE_RELEASE_TAG` is set.
