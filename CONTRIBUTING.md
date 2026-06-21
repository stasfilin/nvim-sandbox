# Contributing

Commits and pull request titles must follow Conventional Commits:

```text
feat(cli): add project selector
fix(docker): preserve bind-mounted paths
perf(state): reduce metadata reads
docs: explain Apple Container setup
```

Release behavior remains in the `v0.x.x` series:

- `feat` and breaking changes increment the minor version.
- `fix` and `perf` increment the patch version.
- `docs`, `test`, `build`, `ci`, `chore`, `refactor`, and `style` do not release by default.

Breaking changes use `!` or a `BREAKING CHANGE:` footer, but still produce a
minor release while the project remains pre-1.0.

Before opening a pull request, run the same aggregate check used by CI:

```sh
make check
```

Targeted integration checks are also available:

```sh
make test-install          # staged and custom-path source installs
make test-e2e-docker       # full Docker sandbox lifecycle
make test-e2e-apple-container
```

Release-package validation runs after CI assembles the cross-platform archives
and Debian packages. It verifies checksums, payloads, metadata, permissions, and
executes packages built for the CI host.

CI is split into explicit dependency stages: lint, static checks, unit tests,
and install tests run independently; successful results unlock cross-platform
builds; Docker E2E and package assembly then run in parallel; package
verification publishes the final artifacts only after both succeed. The release
workflow separately validates the CI-approved commit before semantic-release is
allowed to publish it.
