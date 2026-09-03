#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
helper="$script_dir/prepare-release-notes.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/danser-release-notes-test.XXXXXX")"
cleanup() {
  rm -rf -- "$test_root"
}
trap cleanup EXIT

repo_root="$test_root/repo"
mkdir -p "$repo_root"
git -C "$repo_root" init -q
git -C "$repo_root" config user.email "release-notes-test@example.invalid"
git -C "$repo_root" config user.name "Release Notes Test"

write_fixture() {
  local content=$1
  printf '%s\n' "$content" > "$repo_root/CHANGELOG.md"
}

assert_contains() {
  local file=$1
  local expected=$2
  if ! grep -Fq -- "$expected" "$file"; then
    printf 'expected %s to contain: %s\n' "$file" "$expected" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file=$1
  local unexpected=$2
  if grep -Fq -- "$unexpected" "$file"; then
    printf 'expected %s not to contain: %s\n' "$file" "$unexpected" >&2
    exit 1
  fi
}

expect_failure() {
  if RELEASE_REPO_ROOT="$repo_root" REPOSITORY_URL="https://example.invalid/danser-ee" \
    "$@" >/dev/null 2>&1; then
    printf 'expected command to fail: %s\n' "$*" >&2
    exit 1
  fi
}

write_fixture '# Changelog

## Unreleased

Future work.

## 0.12.0 - 2026-09-02

Short release summary.

## BIGGEST CHANGE

- The complete manually written section is retained.

### Gameplay

- Gameplay notes stay in the release body.

---

## danser 0.12.0 snapshot changes carried forward

- Older snapshot content must not leak into the new body.

## 0.11.0 - 2026-08-01

- Older release content must not leak into the new body.'
git -C "$repo_root" add CHANGELOG.md
git -C "$repo_root" commit -q -m 'test: seed release notes fixture'
git -C "$repo_root" tag 0.11.0

output_file="$test_root/notes.md"
RELEASE_REPO_ROOT="$repo_root" REPOSITORY_URL="https://example.invalid/danser-ee" \
  bash "$helper" 0.12.0 > "$output_file"
assert_contains "$output_file" 'Short release summary.'
assert_contains "$output_file" '## BIGGEST CHANGE'
assert_contains "$output_file" '### Gameplay'
assert_contains "$output_file" 'Full Changelog: [0.11.0...0.12.0](https://example.invalid/danser-ee/compare/0.11.0...0.12.0)'
assert_not_contains "$output_file" 'Older snapshot content must not leak'
assert_not_contains "$output_file" 'Older release content must not leak'
if [[ "$(grep -Fc 'Full Changelog:' "$output_file")" != '1' ]]; then
  printf 'expected exactly one Full Changelog footer\n' >&2
  exit 1
fi

explicit_output="$test_root/explicit-notes.md"
git -C "$repo_root" tag 0.10.0
RELEASE_REPO_ROOT="$repo_root" REPOSITORY_URL="https://example.invalid/danser-ee" \
  bash "$helper" 0.12.0 0.10.0 > "$explicit_output"
assert_contains "$explicit_output" 'Full Changelog: [0.10.0...0.12.0]'

write_fixture '# Changelog

## 1.0.0+build.1 - 2026-09-02

- Build metadata remains valid in the release tag.'
metadata_output="$test_root/metadata-notes.md"
RELEASE_REPO_ROOT="$repo_root" REPOSITORY_URL="https://example.invalid/danser-ee" \
  bash "$helper" 1.0.0+build.1 0.11.0 > "$metadata_output"
assert_contains "$metadata_output" 'Full Changelog: [0.11.0...1.0.0+build.1](https://example.invalid/danser-ee/compare/0.11.0...1.0.0%2Bbuild.1)'

expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$test_root/missing.md" \
  bash "$helper" 0.12.0 0.11.0
expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$repo_root/CHANGELOG.md" \
  bash "$helper" 0.13.0 0.11.0
expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$repo_root/CHANGELOG.md" \
  bash "$helper" 0.12.0 9.9.9

write_fixture '# Changelog

## 0.12.0 - 2026-09-02

## 0.11.0 - 2026-08-01

- Previous release.'
expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$repo_root/CHANGELOG.md" \
  bash "$helper" 0.12.0 0.11.0

write_fixture '# Changelog

## 0.12.0 - 2026-09-02

- First copy.

## 0.12.0 - 2026-09-03

- Duplicate copy.'
expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$repo_root/CHANGELOG.md" \
  bash "$helper" 0.12.0 0.11.0

write_fixture '# Changelog

## 0.12.0

- Missing release date.'
expect_failure env RELEASE_REPO_ROOT="$repo_root" CHANGELOG_FILE="$repo_root/CHANGELOG.md" \
  bash "$helper" 0.12.0 0.11.0

no_tag_repo="$test_root/no-tag-repo"
mkdir -p "$no_tag_repo"
git -C "$no_tag_repo" init -q
printf '%s\n' '# Changelog' '' '## [1.0.0] - 2026-09-02' '' '- No comparison tag exists.' > "$no_tag_repo/CHANGELOG.md"
expect_failure env RELEASE_REPO_ROOT="$no_tag_repo" CHANGELOG_FILE="$no_tag_repo/CHANGELOG.md" \
  bash "$helper" 1.0.0

printf 'release notes helper tests passed\n'
