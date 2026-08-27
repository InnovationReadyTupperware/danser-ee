#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="$repo_root${PATH:+:$PATH}"
export GOCACHE="${GOCACHE:-$repo_root/cache/go-build}"

case "${OSTYPE:-}" in
linux*) export LD_LIBRARY_PATH="$repo_root${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" ;;
darwin*) export DYLD_LIBRARY_PATH="$repo_root${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}" ;;
esac

cd "$repo_root"

if (($# == 0)); then
	set -- ./...
fi

exec go test "$@"
