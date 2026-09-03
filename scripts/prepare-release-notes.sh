#!/usr/bin/env bash

set -euo pipefail

if (( $# < 1 || $# > 2 )); then
  echo "usage: prepare-release-notes.sh VERSION [PREVIOUS_VERSION]" >&2
  exit 2
fi

version=$1
previous_version=${2:-}
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="${RELEASE_REPO_ROOT:-$(cd -- "$script_dir/.." && pwd)}"
changelog_file="${CHANGELOG_FILE:-$repo_root/CHANGELOG.md}"

bash "$script_dir/validate-semver.sh" "$version"
if [[ -n "$previous_version" ]]; then
  bash "$script_dir/validate-semver.sh" "$previous_version"
fi

if [[ ! -f "$changelog_file" ]]; then
  printf 'release notes source not found: %s\n' "$changelog_file" >&2
  exit 1
fi

mapfile -t matching_headers < <(
  awk -v version="$version" '
    function is_target(line, prefix, bracketed_prefix, date) {
      prefix = "## " version " - "
      bracketed_prefix = "## [" version "] - "
      if (index(line, prefix) != 1 && index(line, bracketed_prefix) != 1) {
        return 0
      }

      if (index(line, prefix) == 1 && length(line) != length(prefix) + 10) {
        return 0
      }
      if (index(line, bracketed_prefix) == 1 && length(line) != length(bracketed_prefix) + 10) {
        return 0
      }

      date = substr(line, length(line) - 9)
      return date ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/
    }

    {
      line = $0
      sub(/\r$/, "", line)
      if (is_target(line)) {
        print NR
      }
    }
  ' "$changelog_file"
)

if (( ${#matching_headers[@]} == 0 )); then
  printf 'release notes entry not found for version %s in %s\n' "$version" "$changelog_file" >&2
  printf 'expected a heading like: ## %s - YYYY-MM-DD\n' "$version" >&2
  exit 1
fi

if (( ${#matching_headers[@]} > 1 )); then
  printf 'release notes entry is duplicated for version %s in %s\n' "$version" "$changelog_file" >&2
  exit 1
fi

start_line=${matching_headers[0]}
next_boundary="$(
  awk -v start_line="$start_line" '
    function is_boundary(line, date) {
      if (line == "---") {
        return 1
      }
      if (line == "## Unreleased" || line == "## [Unreleased]") {
        return 1
      }
      if (line !~ /^## / || line !~ / - / || length(line) < 10) {
        return 0
      }

      date = substr(line, length(line) - 9)
      return date ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/
    }

    NR > start_line {
      line = $0
      sub(/\r$/, "", line)
      if (is_boundary(line)) {
        print NR
        exit
      }
    }
  ' "$changelog_file"
)"

body_file="$(mktemp "${TMPDIR:-/tmp}/danser-release-notes.XXXXXX")"
cleanup() {
  rm -f -- "$body_file"
}
trap cleanup EXIT

if [[ -n "$next_boundary" ]]; then
  sed -n "$((start_line + 1)),$((next_boundary - 1))p" "$changelog_file" > "$body_file"
else
  sed -n "$((start_line + 1)),\$p" "$changelog_file" > "$body_file"
fi

if ! grep -q '[^[:space:]]' "$body_file"; then
  printf 'release notes entry is empty for version %s in %s\n' "$version" "$changelog_file" >&2
  exit 1
fi

if ! git -C "$repo_root" rev-parse --git-dir >/dev/null 2>&1; then
  printf 'release notes comparison requires a Git repository: %s\n' "$repo_root" >&2
  exit 1
fi

if [[ -z "$previous_version" ]]; then
  previous_version="$(
    mapfile -t candidate_tags < <(git -C "$repo_root" tag --merged HEAD --sort=-version:refname)
    for tag in "${candidate_tags[@]}"; do
      [[ "$tag" == "$version" ]] && continue
      if bash "$script_dir/validate-semver.sh" "$tag" >/dev/null 2>&1; then
        printf '%s\n' "$tag"
        break
      fi
    done
  )"
fi

if [[ -z "$previous_version" ]]; then
  printf 'could not determine a previous SemVer tag for %s\n' "$version" >&2
  printf 'provide PREVIOUS_VERSION explicitly when invoking the helper or workflow\n' >&2
  exit 1
fi

if [[ "$previous_version" == "$version" ]]; then
  printf 'previous version must differ from release version %s\n' "$version" >&2
  exit 1
fi

if ! git -C "$repo_root" rev-parse --verify --quiet "refs/tags/$previous_version^{commit}" >/dev/null; then
  printf 'previous release tag does not exist in %s: %s\n' "$repo_root" "$previous_version" >&2
  exit 1
fi

repository_url="${REPOSITORY_URL:-}"
if [[ -z "$repository_url" && -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" ]]; then
  repository_url="${GITHUB_SERVER_URL%/}/${GITHUB_REPOSITORY}"
fi
if [[ -z "$repository_url" ]]; then
  repository_url="https://github.com/InnovationReadyTupperware/danser-ee"
fi
repository_url="${repository_url%/}"

# SemVer build metadata is valid in the tag but the plus sign must be escaped
# in a URL path component.
previous_path=${previous_version//+/%2B}
version_path=${version//+/%2B}

cat "$body_file"
if [[ -n "$(tail -c 1 "$body_file")" ]]; then
  printf '\n'
fi
printf '\nFull Changelog: [%s...%s](%s/compare/%s...%s)\n' \
  "$previous_version" "$version" "$repository_url" "$previous_path" "$version_path"
