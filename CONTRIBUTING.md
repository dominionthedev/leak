# Contributing

This is currently a single-maintainer project. Issues and small,
clearly-scoped PRs are welcome; open an issue first for anything
structural before sending a PR, so the direction is settled before the
code is.

## Before sending a PR

```bash
make check
```

Runs `go build`, `go vet`, a `gofmt -l` check, and `go test -race` for
every package. All four have to pass clean — this is exactly what CI
runs, so if `make check` is green locally, CI will be too.

## Commit messages

Conventional commits, kept short: `type(scope): what changed` — e.g.
`fix(parser): handle empty CSI params`. Add a body explaining *why* if
the summary line doesn't already make that obvious. No narrative or
changelog-style prose in the subject line.

## Scope discipline

A fix-only PR should stay a fix-only PR — no incidental new
architecture, new files, or new dependencies riding along, even if
related. That belongs in its own issue/PR. Bundling unrelated changes
into a "fix" makes it hard to tell what's actually being tested and
reviewed, and makes a bad change hard to revert cleanly later.

## Tests

New behavior needs a test that would fail without the change. For bug
fixes especially: if you can't write a test that reproduces the bug,
say so in the PR description rather than skipping it — a fix without a
regression test is a fix that can silently come back.
