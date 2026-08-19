# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

### Fixed

- Download the `architect` binary from its current release asset in `config/setup.sh`, fixing
  `e2e-tests`. The script resolves `architect`'s latest release and then fetched
  `architect-${VERSION}-linux-amd64.tar.gz`. architect now attaches bare binaries instead: v8.2.x
  published both, and v8.3.0 (2026-07-14) dropped the tarball, so the download has returned 404 ever
  since `latest` became v8.3.0. With no `./architect`, `architect project version` produced an empty
  `TAG`, and the only symptom was `Error: context deadline exceeded` from the `helm --wait` at the end
  of the script, because the image reference had no tag. Also fail fast on an empty tag so the next
  asset rename reports itself instead of surfacing as a helm timeout.

## [0.1.2] - 2026-05-29

### Changed

- Updating Flux OCIRepository versions to v1.

## [0.1.1] - 2026-03-26

### Added

- Add `GenerationChangedPredicate` predicate to avoid reconciliation on status changes.

## [0.1.0] - 2026-03-19

### Added

- Initial release.



[Unreleased]: https://github.com/giantswarm/team-stamper/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/giantswarm/team-stamper/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/team-stamper/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/team-stamper/releases/tag/v0.1.0
