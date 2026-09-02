#!/usr/bin/env bash
# Runs everything a PR needs to pass. Used by both `make check` and CI —
# one source of truth, so "passes locally" and "passes in CI" mean the
# same thing.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> build"
go build ./...

echo "==> vet"
go vet ./...

echo "==> gofmt"
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
	echo "gofmt needs to be run on:"
	echo "$unformatted"
	exit 1
fi

echo "==> test"
go test ./... -race

echo "all checks passed"
