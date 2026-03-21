#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
RUN_TESTS="${RUN_TESTS:-1}"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is not clean. Commit/stash changes before release tagging."
  exit 1
fi

git fetch origin
git rev-parse --abbrev-ref HEAD
git status -sb

if [[ "$RUN_TESTS" == "1" ]]; then
  go test ./...
  (cd frontend && npm run build)
fi

if [[ -n "$TAG" ]]; then
  git tag -a "$TAG" -m "ClientRoll production release $TAG"
  echo "Created annotated tag: $TAG"
  echo "Push with: git push origin $TAG"
else
  echo "No tag argument provided. Pass a tag to create one, e.g. ./scripts/release-gate.sh v2026.03.06-1"
fi
