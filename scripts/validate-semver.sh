#!/bin/bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: validate-semver.sh VERSION" >&2
  exit 2
fi

version=$1
semver_regex='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'

if [[ ! "$version" =~ $semver_regex ]]; then
  printf 'invalid SemVer 2.0.0 release version: %s\n' "$version" >&2
  exit 1
fi
